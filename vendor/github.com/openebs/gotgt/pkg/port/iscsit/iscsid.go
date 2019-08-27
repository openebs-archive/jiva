/*
Copyright 2017 The GoStor Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package iscsit

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/openebs/gotgt/pkg/api"
	"github.com/openebs/gotgt/pkg/config"
	"github.com/openebs/gotgt/pkg/scsi"
	"github.com/openebs/gotgt/pkg/util"
	"github.com/openebs/gotgt/pkg/util/pool"
)

const (
	ISCSI_MAX_TSIH    = uint16(0xffff)
	ISCSI_UNSPEC_TSIH = uint16(0)
)

type operation func(*iscsiConnection) error

type ISCSITargetDriver struct {
	SCSI              *scsi.SCSITargetService
	Name              string
	iSCSITargets      map[string]*ISCSITarget
	done              chan bool
	stopFeed          chan bool
	feed              chan Msg
	statsEnq          chan bool
	isClientConnected bool
	statsEnqRes       chan scsi.Stats
	SCSIIOCount       map[int]int64
	listen            net.Listener
	clusterIP         string
	state             uint8
	lock              *sync.RWMutex
	TSIHPool          map[uint16]bool
	TSIHPoolMutex     sync.Mutex
}

func init() {
	scsi.RegisterTargetDriver(iSCSIDriverName, NewISCSITargetDriver)
}

func (s *ISCSITargetDriver) AllocTSIH() uint16 {
	var i uint16
	s.TSIHPoolMutex.Lock()
	for i = uint16(0); i < ISCSI_MAX_TSIH; i++ {
		exist := s.TSIHPool[i]
		if !exist {
			s.TSIHPool[i] = true
			s.TSIHPoolMutex.Unlock()
			return i
		}
	}
	s.TSIHPoolMutex.Unlock()
	return ISCSI_UNSPEC_TSIH
}

func (s *ISCSITargetDriver) ReleaseTSIH(tsih uint16) {
	s.TSIHPoolMutex.Lock()
	delete(s.TSIHPool, tsih)
	s.TSIHPoolMutex.Unlock()
}

const (
	STATE_INIT = iota
	STATE_RUNNING
	STATE_SHUTTING_DOWN
	STATE_TERMINATE
)

var (
	opPingThreshold = 3 * time.Second
)

func NewISCSITargetDriver(base *scsi.SCSITargetService) (scsi.SCSITargetDriver, error) {
	return &ISCSITargetDriver{
		Name:         iSCSIDriverName,
		iSCSITargets: map[string]*ISCSITarget{},
		SCSI:         base,
		done:         make(chan bool, 1),
		stopFeed:     make(chan bool, 1),
		feed:         make(chan Msg, 10000),
		statsEnq:     make(chan bool, 1),
		statsEnqRes:  make(chan scsi.Stats, 1),
		SCSIIOCount:  map[int]int64{},
		state:        STATE_INIT,
		lock:         &sync.RWMutex{},
		TSIHPool:     map[uint16]bool{0: true, 65535: true},
	}, nil
}

func timed(f operation, conn *iscsiConnection) {
	start := time.Now()
	f(conn)
	since := time.Since(start)
	if since > opPingThreshold {
		log.Warningf("Ping request took %vs at target", since.Seconds())
	}
}

func (s *ISCSITargetDriver) getState() uint8 {
	s.lock.Lock()
	defer s.lock.Unlock()

	return s.state
}

func (s *ISCSITargetDriver) setState(st uint8) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.state = st
}
func (s *ISCSITargetDriver) Stats() scsi.Stats {
	//s.lock.Lock()
	//defer s.lock.Unlock()

	s.statsEnq <- true
	stats := <-s.statsEnqRes
	stats.SCSIIOCount = s.SCSIIOCount
	return stats
}

func (s *ISCSITargetDriver) Resize(size uint64) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	return s.SCSI.Resize(size)

}

func (s *ISCSITargetDriver) SetClusterIP(clusterIP string) error {
	s.clusterIP = clusterIP
	return nil
}

func (s *ISCSITargetDriver) NewTarget(tgtName string, configInfo *config.Config) error {
	if _, ok := s.iSCSITargets[tgtName]; ok {
		return fmt.Errorf("target name has been existed")
	}
	stgt, err := s.SCSI.NewSCSITarget(len(s.iSCSITargets), "iscsi", tgtName)
	if err != nil {
		return err
	}
	tgt := newISCSITarget(stgt)

	s.iSCSITargets[tgtName] = tgt
	scsiTPG := tgt.SCSITarget.TargetPortGroups[0]
	targetConfig := configInfo.ISCSITargets[tgtName]
	for tpgt, portalIDArrary := range targetConfig.TPGTs {
		tpgtNumber, _ := strconv.ParseUint(tpgt, 10, 16)
		tgt.TPGTs[uint16(tpgtNumber)] = &iSCSITPGT{uint16(tpgtNumber), make(map[string]struct{})}
		targetPortName := fmt.Sprintf("%s,t,0x%02x", tgtName, tpgtNumber)
		scsiTPG.TargetPortGroup = append(scsiTPG.TargetPortGroup, &api.SCSITargetPort{uint16(tpgtNumber), targetPortName})
		for _, portalID := range portalIDArrary {
			portal := configInfo.ISCSIPortals[portalID]
			s.AddiSCSIPortal(tgtName, uint16(tpgtNumber), portal.Portal)
		}
	}
	return nil
}

func (s *ISCSITargetDriver) AddiSCSIPortal(tgtName string, tpgt uint16, portal string) error {
	var (
		ok       bool
		target   *ISCSITarget
		tpgtInfo *iSCSITPGT
	)

	if target, ok = s.iSCSITargets[tgtName]; !ok {
		return fmt.Errorf("No such target: %s", tgtName)
	}

	if tpgtInfo, ok = target.TPGTs[tpgt]; !ok {
		return fmt.Errorf("No such TPGT: %d", tpgt)
	}
	tgtPortals := tpgtInfo.Portals

	if _, ok = tgtPortals[portal]; !ok {
		tgtPortals[portal] = struct{}{}
	} else {
		return fmt.Errorf("duplicate portal %s,in %s,%d", portal, tgtName, tpgt)
	}

	return nil
}

func (s *ISCSITargetDriver) HasPortal(tgtName string, tpgt uint16, portal string) bool {
	var (
		ok       bool
		target   *ISCSITarget
		tpgtInfo *iSCSITPGT
	)

	if target, ok = s.iSCSITargets[tgtName]; !ok {
		return false
	}
	if tpgtInfo, ok = target.TPGTs[tpgt]; !ok {
		return false
	}
	tgtPortals := tpgtInfo.Portals

	if _, ok = tgtPortals[portal]; !ok {
		return false
	} else {
		return true
	}
}

// Stop the server by closing the listener
// todo: handling established connections
func (s *ISCSITargetDriver) Stop() error {
	if s.getState() != STATE_RUNNING {
		log.Infof("SKIP Closing Listener...  state is %v\n", s.getState())
		return nil
	}

	s.setState(STATE_SHUTTING_DOWN)
	s.done <- true
	log.Info("Closing Listener...")
	s.listen.Close() // Accept will see an error
	s.setState(STATE_TERMINATE)

	return nil
}

func (s *ISCSITargetDriver) Run() error {
	var (
		err       error
		connNum   int
		iscsiConn *iscsiConnection
	)
	for _, iSCSITarget := range s.iSCSITargets {
		for _, iSCSITPGT := range iSCSITarget.TPGTs {
			for portal, _ := range iSCSITPGT.Portals {
				if portal != "" {
					s.listen, err = net.Listen("tcp", portal)
				}
			}
		}
	}
	log.Infof("iSCSI service listening on: %v", s.listen.Addr())
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
	defer func() {
		s.setState(STATE_SHUTTING_DOWN)
		s.stopFeed <- true
		s.isClientConnected = false
		s.setState(STATE_TERMINATE)
	}()
	s.setState(STATE_RUNNING)
	go s.StatsFeed()
	for {
		conn, err := s.listen.Accept()
		if err != nil {
			select {
			case <-s.done:
				if iscsiConn != nil {
					log.Warning("Closing connection with initiator...")
					iscsiConn.conn.Close()
					iscsiConn = nil
				}
			default:
				log.Error(err)

			}
			return err
		}
		s.SetClientStatus(true)

		iscsiConn = &iscsiConnection{conn: conn,
			loginParam: &iscsiLoginParam{},
		}

		iscsiConn.init()
		iscsiConn.ConnNum = connNum
		connNum += 1
		iscsiConn.rxIOState = IOSTATE_RX_BHS

		log.Infof("Target is connected to initiator: %s", conn.RemoteAddr().String())
		// start a new thread to do with this command
		go s.handler(DATAIN, iscsiConn)
	}
	s.stopFeed <- true
	return nil
}

func (s *ISCSITargetDriver) SetClientStatus(ok bool) {
	s.isClientConnected = ok
}

func (s *ISCSITargetDriver) IsClientConnected() bool {
	return s.isClientConnected
}

func (s *ISCSITargetDriver) handler(events byte, conn *iscsiConnection) {

	if events&DATAIN != 0 {
		log.Debug("rx handler processing...")
		go s.rxHandler(conn)
	}
	if conn.state != CONN_STATE_CLOSE && events&DATAOUT != 0 {
		log.Debug("tx handler processing...")
		s.txHandler(conn)
	}
	if conn.state == CONN_STATE_CLOSE {
		log.Warningf("iscsi connection[%d] closed", conn.cid)
		conn.close()
	}
}

func (s *ISCSITargetDriver) rxHandler(conn *iscsiConnection) {
	var (
		hdigest uint = 0
		ddigest uint = 0
		final   bool = false
		cmd     *ISCSICommand
		buf     []byte = make([]byte, BHS_SIZE)
		length  int
		err     error
	)
	conn.readLock.Lock()
	defer conn.readLock.Unlock()
	if conn.state == CONN_STATE_SCSI {
		hdigest = conn.loginParam.sessionParam[ISCSI_PARAM_HDRDGST_EN].Value & DIGEST_CRC32C
		ddigest = conn.loginParam.sessionParam[ISCSI_PARAM_DATADGST_EN].Value & DIGEST_CRC32C
	}
	for {
		switch conn.rxIOState {
		case IOSTATE_RX_BHS:
			log.Debug("rx handler: IOSTATE_RX_BHS")
			length, err = conn.readData(buf)
			if err != nil {
				log.Error(err)
				return
			}
			if length == 0 {
				log.Warningf("set connection to close")
				conn.state = CONN_STATE_CLOSE
				return
			}
			cmd, err = parseHeader(buf)
			if err != nil {
				log.Error(err)
				log.Warningf("set connection to close")
				conn.state = CONN_STATE_CLOSE
				return
			}
			conn.req = cmd
			if length == BHS_SIZE && cmd.DataLen != 0 {
				conn.rxIOState = IOSTATE_RX_INIT_AHS
				break
			}
			if log.GetLevel() == log.DebugLevel {
				log.Debugf("got command: \n%s", cmd.String())
				log.Debugf("got buffer: %v", buf)
			}
			final = true
		case IOSTATE_RX_INIT_AHS:
			conn.rxIOState = IOSTATE_RX_DATA
			break
			if hdigest != 0 {
				conn.rxIOState = IOSTATE_RX_INIT_HDIGEST
			}
		case IOSTATE_RX_DATA:
			if ddigest != 0 {
				conn.rxIOState = IOSTATE_RX_INIT_DDIGEST
			}
			if cmd == nil {
				return
			}
			dl := ((cmd.DataLen + DataPadding - 1) / DataPadding) * DataPadding
			cmd.RawData = pool.NewBuffer(dl)
			length := 0
			for length < dl {
				l, err := conn.readData(cmd.RawData[length:])
				if err != nil {
					log.Error(err)
					return
				}
				length += l
			}
			if length != dl {
				log.Debugf("get length is %d, but expected %d", length, dl)
				log.Warning("set connection to close")
				conn.state = CONN_STATE_CLOSE
				return
			}
			final = true
		default:
			log.Errorf("error %d %d\n", conn.state, conn.rxIOState)
			return
		}

		if final {
			break
		}
	}

	if conn.state == CONN_STATE_SCSI {
		s.scsiCommandHandler(conn)
	} else {
		conn.txIOState = IOSTATE_TX_BHS
		conn.resp = &ISCSICommand{}
		switch conn.req.OpCode {
		case OpLoginReq:
			log.Debug("OpLoginReq")
			if err := s.iscsiExecLogin(conn); err != nil {
				log.Error(err)
				log.Warningf("set connection to close")
				conn.state = CONN_STATE_CLOSE
			}
		case OpLogoutReq:
			log.Debug("OpLogoutReq")
			s.SetClientStatus(false)
			if err := iscsiExecLogout(conn); err != nil {
				log.Warningf("set connection to close")
				conn.state = CONN_STATE_CLOSE
			}
		case OpTextReq:
			log.Debug("OpTextReq")
			if err := s.iscsiExecText(conn); err != nil {
				log.Warningf("set connection to close")
				conn.state = CONN_STATE_CLOSE
			}
		default:
			iscsiExecReject(conn)
		}
		log.Debugf("connection state is %v", conn.State())
		s.handler(DATAOUT, conn)
	}
}

func (s *ISCSITargetDriver) iscsiExecLogin(conn *iscsiConnection) error {
	var cmd = conn.req

	conn.cid = cmd.ConnID
	conn.loginParam.iniCSG = cmd.CSG
	conn.loginParam.iniNSG = cmd.NSG
	conn.loginParam.iniCont = cmd.Cont
	conn.loginParam.iniTrans = cmd.Transit
	conn.loginParam.isid = cmd.ISID
	conn.loginParam.tsih = cmd.TSIH
	conn.expCmdSN = cmd.CmdSN
	conn.maxBurstLength = MaxBurstLength
	conn.maxRecvDataSegmentLength = MaxRecvDataSegmentLength
	conn.maxSeqCount = conn.maxBurstLength / conn.maxRecvDataSegmentLength

	if conn.loginParam.iniCSG == SecurityNegotiation {
		conn.state = CONN_STATE_EXIT
		return fmt.Errorf("Doesn't support Auth")
	}

	_, err := conn.processLoginData()
	if err != nil {
		return err
	}

	if !conn.loginParam.paramInit {
		err = s.BindISCSISession(conn)
		if err != nil {
			conn.state = CONN_STATE_EXIT
			return err
		}
		conn.loginParam.paramInit = true
	}
	if conn.loginParam.tgtNSG == FullFeaturePhase &&
		conn.loginParam.tgtTrans {
		conn.state = CONN_STATE_LOGIN_FULL
	} else {
		conn.state = CONN_STATE_LOGIN
	}

	return conn.buildRespPackage(OpLoginResp, nil)
}

func iscsiExecLogout(conn *iscsiConnection) error {
	if conn.conn != nil {
		log.Infof("Logout request received from initiator: %v", conn.conn.RemoteAddr().String())
	}
	cmd := conn.req
	conn.resp = &ISCSICommand{
		OpCode:  OpLogoutResp,
		StatSN:  cmd.ExpStatSN,
		TaskTag: cmd.TaskTag,
	}
	if conn.session == nil {
		conn.resp.ExpCmdSN = cmd.CmdSN
		conn.resp.MaxCmdSN = cmd.CmdSN
	} else {
		conn.resp.ExpCmdSN = conn.session.ExpCmdSN
		conn.resp.MaxCmdSN = conn.session.ExpCmdSN + conn.session.MaxQueueCommand
	}
	return nil
}

func (s *ISCSITargetDriver) iscsiExecText(conn *iscsiConnection) error {
	var result = []util.KeyValue{}
	cmd := conn.req
	keys := util.ParseKVText(cmd.RawData)
	if st, ok := keys["SendTargets"]; ok {
		if st == "All" {
			for name, tgt := range s.iSCSITargets {
				log.Debugf("iscsi target: %v", name)
				//log.Debugf("iscsi target portals: %v", tgt.Portals)

				result = append(result, util.KeyValue{"TargetName", name})
				if s.clusterIP == "" {
					for _, tpgt := range tgt.TPGTs {
						for portal := range tpgt.Portals {
							targetPort := fmt.Sprintf("%s,%d", portal, tpgt.TPGT)
							result = append(result, util.KeyValue{"TargetAddress", targetPort})
						}
					}
				} else {
					for _, tpgt := range tgt.TPGTs {
						targetPort := fmt.Sprintf("%s,%d", s.clusterIP, tpgt.TPGT)
						result = append(result, util.KeyValue{"TargetAddress", targetPort})
					}
				}
			}
		}
	}

	conn.resp = &ISCSICommand{
		OpCode:   OpTextResp,
		Final:    true,
		NSG:      FullFeaturePhase,
		StatSN:   cmd.ExpStatSN,
		TaskTag:  cmd.TaskTag,
		ExpCmdSN: cmd.CmdSN,
		MaxCmdSN: cmd.CmdSN,
	}
	conn.resp.RawData = util.MarshalKVText(result)
	return nil
}

func iscsiExecNoopOut(conn *iscsiConnection) error {
	return conn.buildRespPackage(OpNoopIn, nil)
}

func iscsiExecReject(conn *iscsiConnection) error {
	return conn.buildRespPackage(OpReject, nil)
}

func iscsiExecR2T(conn *iscsiConnection) error {
	return conn.buildRespPackage(OpReady, nil)
}

type Msg struct {
	ConnNum     int
	OpCode      int
	BlockCount  int64
	ElapsedTime time.Duration
}

func (s *ISCSITargetDriver) StatsFeed() {
	var (
		ReadIOPS            int64
		TotalReadTime       int64
		TotalReadBlockCount int64

		WriteIOPS            int64
		TotalWriteTime       int64
		TotalWriteBlockCount int64
	)

	for {
		select {
		case <-s.stopFeed:
			return
		case msg := <-s.feed:
			switch api.SCSICommandType(msg.OpCode) {
			case api.READ_6, api.READ_10, api.READ_12, api.READ_16:
				ReadIOPS += 1
				TotalReadTime += int64(msg.ElapsedTime)
				TotalReadBlockCount += msg.BlockCount
				break
			case api.WRITE_6, api.WRITE_10, api.WRITE_12, api.WRITE_16:
				WriteIOPS += 1
				TotalWriteTime += int64(msg.ElapsedTime)
				TotalWriteBlockCount += msg.BlockCount
				break
			}
		case <-s.statsEnq:
			s.statsEnqRes <- scsi.Stats{
				IsClientConnected:   s.IsClientConnected(),
				ReadIOPS:            ReadIOPS,
				TotalReadBlockCount: TotalReadBlockCount,
				TotalReadTime:       TotalReadTime,

				WriteIOPS:            WriteIOPS,
				TotalWriteBlockCount: TotalWriteBlockCount,
				TotalWriteTime:       TotalWriteTime,
			}
		}
	}
}

func (s *ISCSITargetDriver) txHandler(conn *iscsiConnection) {
	var (
		hdigest uint   = 0
		ddigest uint   = 0
		offset  uint32 = 0
		final   bool   = false
		count   uint32 = 0
		msg     Msg
	)
	if conn.state == CONN_STATE_SCSI {
		hdigest = conn.loginParam.sessionParam[ISCSI_PARAM_HDRDGST_EN].Value & DIGEST_CRC32C
		ddigest = conn.loginParam.sessionParam[ISCSI_PARAM_DATADGST_EN].Value & DIGEST_CRC32C
	}
	if conn.state == CONN_STATE_SCSI && conn.txTask == nil {
		err := s.scsiCommandHandler(conn)
		if err != nil {
			log.Error(err)
			return
		}
	}
	resp := conn.resp
	segmentLen := conn.maxRecvDataSegmentLength
	transferLen := len(resp.RawData)
	resp.DataSN = 0
	maxCount := conn.maxSeqCount

	if resp.OpCode == OpSCSIResp || resp.OpCode == OpSCSIIn {
		msg.ConnNum = conn.ConnNum
		msg.OpCode = int(resp.SCSIOpCode)
		msg.ElapsedTime = time.Since(resp.StartTime)
		msg.BlockCount = int64(resp.ExpectedDataLen)
		s.feed <- msg
	}

	/* send data splitted by segmentLen */
SendRemainingData:
	if resp.OpCode == OpSCSIIn {
		resp.BufferOffset = offset
		if int(offset+segmentLen) < transferLen {
			count += 1
			if count < maxCount {
				resp.FinalInSeq = false
				resp.Final = false
			} else {
				count = 0
				resp.FinalInSeq = true
				resp.Final = false
			}
			offset = offset + segmentLen
			resp.DataLen = int(segmentLen)
		} else {
			resp.FinalInSeq = true
			resp.Final = true
			resp.DataLen = transferLen - int(offset)
		}
	}
	for {
		switch conn.txIOState {
		case IOSTATE_TX_BHS:
			if log.GetLevel() == log.DebugLevel {
				log.Debug("ready to write response")
				log.Debugf("response is %s", resp.String())
			}
			if l, err := conn.write(resp.Bytes()); err != nil {
				log.Errorf("failed to write data to client: %v", err)
				return
			} else {
				conn.txIOState = IOSTATE_TX_INIT_AHS
				log.Debugf("success to write %d length", l)
			}
		case IOSTATE_TX_INIT_AHS:
			if hdigest != 0 {
				conn.txIOState = IOSTATE_TX_INIT_HDIGEST
			} else {
				conn.txIOState = IOSTATE_TX_INIT_DATA
			}
			if conn.txIOState != IOSTATE_TX_AHS {
				final = true
			}
		case IOSTATE_TX_AHS:
		case IOSTATE_TX_INIT_DATA:
			final = true
		case IOSTATE_TX_DATA:
			if ddigest != 0 {
				conn.txIOState = IOSTATE_TX_INIT_DDIGEST
			}
		default:
			log.Errorf("error %d %d\n", conn.state, conn.txIOState)
			return
		}

		if final {
			if resp.OpCode == OpSCSIIn && resp.Final != true {
				resp.DataSN++
				conn.txIOState = IOSTATE_TX_BHS
				goto SendRemainingData
			} else {
				break
			}
		}
	}

	log.Debugf("connection state: %v", conn.State())
	switch conn.state {
	case CONN_STATE_CLOSE, CONN_STATE_EXIT:
		conn.state = CONN_STATE_CLOSE
	case CONN_STATE_SECURITY_LOGIN:
		conn.state = CONN_STATE_LOGIN
	case CONN_STATE_LOGIN:
		conn.rxIOState = IOSTATE_RX_BHS
		s.handler(DATAIN, conn)
	case CONN_STATE_SECURITY_FULL, CONN_STATE_LOGIN_FULL:
		if conn.session.SessionType == SESSION_NORMAL {
			conn.state = CONN_STATE_KERNEL
			conn.state = CONN_STATE_SCSI
		} else {
			conn.state = CONN_STATE_FULL
		}
		conn.rxIOState = IOSTATE_RX_BHS
		s.handler(DATAIN, conn)
	case CONN_STATE_SCSI:
		conn.txTask = nil
	default:
		log.Warnf("unexpected connection state: %v", conn.State())
		conn.rxIOState = IOSTATE_RX_BHS
		s.handler(DATAIN, conn)
	}
}

func (s *ISCSITargetDriver) scsiCommandHandler(conn *iscsiConnection) (err error) {
	req := conn.req
	switch req.OpCode {
	case OpSCSICmd:
		log.Debugf("SCSI Command processing...")
		if _, ok := s.SCSIIOCount[(int)(req.CDB[0])]; ok != false {
			s.SCSIIOCount[(int)(req.CDB[0])] += 1
		} else {
			s.SCSIIOCount[(int)(req.CDB[0])] = 1
		}
		scmd := &api.SCSICommand{
			ITNexusID:       conn.session.ITNexus.ID,
			SCB:             req.CDB,
			SCBLength:       len(req.CDB),
			Lun:             req.LUN,
			Tag:             uint64(req.TaskTag),
			RelTargetPortID: conn.session.TPGT,
		}
		if req.Read {
			if req.Write {
				scmd.Direction = api.SCSIDataBidirection
			} else {
				scmd.Direction = api.SCSIDataRead
			}
		} else {
			if req.Write {
				scmd.Direction = api.SCSIDataWrite
			}
		}

		task := &iscsiTask{conn: conn, cmd: conn.req, tag: conn.req.TaskTag, scmd: scmd}
		task.scmd.OpCode = conn.req.SCSIOpCode
		if scmd.Direction == api.SCSIDataBidirection {
			task.scmd.Result = api.SAMStatCheckCondition.Stat
			scsi.BuildSenseData(task.scmd, scsi.ILLEGAL_REQUEST, scsi.NO_ADDITIONAL_SENSE)
			conn.buildRespPackage(OpSCSIResp, task)
			conn.rxTask = nil
			break
		}
		if req.Write {
			task.expectedDataLength = int64(req.ExpectedDataLen)
			task.r2tCount = int(req.ExpectedDataLen) - req.DataLen
			if !req.Final {
				task.unsolCount = 1
			}
			// new buffer for the data out
			if scmd.OutSDBBuffer == nil {
				blen := int(req.ExpectedDataLen)
				if blen == 0 {
					blen = int(req.DataLen)
				}
				scmd.OutSDBBuffer = &api.SCSIDataBuffer{
					Length: uint32(blen),
					Buffer: pool.NewBuffer(blen),
				}
			}
			log.Debugf("SCSI write, R2T count: %d, unsol Count: %d, offset: %d", task.r2tCount, task.unsolCount, task.offset)

			if conn.session.SessionParam[ISCSI_PARAM_IMM_DATA_EN].Value == 1 {
				copy(scmd.OutSDBBuffer.Buffer[task.offset:], conn.req.RawData)
				task.offset += conn.req.DataLen
			}
			if task.r2tCount > 0 {
				// prepare to receive more data
				conn.session.ExpCmdSN += 1
				task.state = taskPending
				conn.session.PendingTasksMutex.Lock()
				conn.session.PendingTasks.Push(task)
				conn.session.PendingTasksMutex.Unlock()
				conn.rxTask = task
				if conn.session.SessionParam[ISCSI_PARAM_INITIAL_R2T_EN].Value == 1 {
					iscsiExecR2T(conn)
					break
				} else {
					log.Debugf("Not ready to exec the task")
					conn.rxIOState = IOSTATE_RX_BHS
					s.handler(DATAIN, conn)
					return nil
				}
			}
		} else if scmd.InSDBBuffer == nil {
			scmd.InSDBBuffer = &api.SCSIDataBuffer{
				Length: uint32(req.ExpectedDataLen),
				Buffer: pool.NewBuffer(int(req.ExpectedDataLen)),
			}
		}
		task.offset = 0
		conn.rxTask = task
		if err = s.iscsiTaskQueueHandler(task); err != nil {
			if task.state == taskPending {
				s.handler(DATAIN, conn)
				err = nil
			}
			return
		} else {
			if scmd.Direction == api.SCSIDataRead && scmd.SenseBuffer == nil && req.ExpectedDataLen != 0 {
				conn.buildRespPackage(OpSCSIIn, task)
			} else {
				conn.buildRespPackage(OpSCSIResp, task)
			}
			conn.rxTask = nil
		}
	case OpSCSITaskReq:
		// task management function
		task := &iscsiTask{conn: conn, cmd: conn.req, tag: conn.req.TaskTag, scmd: nil}
		conn.rxTask = task
		if err = s.iscsiTaskQueueHandler(task); err != nil {
			return
		}
	case OpSCSIOut:
		log.Debugf("iSCSI Data-out processing...")
		var task, t *iscsiTask
		conn.session.PendingTasksMutex.Lock()
		for _, t = range conn.session.PendingTasks {
			if t.tag == conn.req.TaskTag {
				task = t
				break
			}
		}
		conn.session.PendingTasksMutex.Unlock()
		if task == nil {
			err = fmt.Errorf("Cannot find iSCSI task with tag[%v]", conn.req.TaskTag)
			log.Error(err)
			return
		}
		copy(task.scmd.OutSDBBuffer.Buffer[task.offset:], conn.req.RawData)
		task.offset += conn.req.DataLen
		task.r2tCount = task.r2tCount - conn.req.DataLen
		log.Debugf("Final: %v", conn.req.Final)
		log.Debugf("r2tCount: %v", task.r2tCount)
		if !conn.req.Final {
			log.Debugf("Not ready to exec the task")
			conn.rxIOState = IOSTATE_RX_BHS
			s.handler(DATAIN, conn)
			return nil
		} else if task.r2tCount > 0 {
			// prepare to receive more data
			if task.unsolCount == 0 {
				task.r2tSN += 1
			} else {
				task.r2tSN = 0
				task.unsolCount = 0
			}
			conn.rxTask = task
			iscsiExecR2T(conn)
			break
		}
		task.offset = 0
		log.Debugf("Process the Data-out package")
		conn.rxTask = task
		if err = s.iscsiExecTask(task); err != nil {
			return
		} else {
			conn.buildRespPackage(OpSCSIResp, task)
			conn.rxTask = nil
			conn.session.PendingTasksMutex.Lock()
			conn.session.PendingTasks.Delete(task)
			conn.session.PendingTasksMutex.Unlock()
		}
	case OpNoopOut:
		timed(iscsiExecNoopOut, conn)
	case OpLogoutReq:
		conn.txTask = &iscsiTask{conn: conn, cmd: conn.req, tag: conn.req.TaskTag}
		conn.txIOState = IOSTATE_TX_BHS
		s.SetClientStatus(false)
		iscsiExecLogout(conn)
	case OpTextReq, OpSNACKReq:
		err = fmt.Errorf("Cannot handle yet %s", opCodeMap[conn.req.OpCode])
		log.Error(err)
		return
	default:
		err = fmt.Errorf("Unknown op %s", opCodeMap[conn.req.OpCode])
		log.Error(err)
		return
	}
	conn.rxIOState = IOSTATE_RX_BHS
	s.handler(DATAIN|DATAOUT, conn)
	return nil
}

func (s *ISCSITargetDriver) iscsiTaskQueueHandler(task *iscsiTask) error {
	conn := task.conn
	sess := conn.session
	cmd := task.cmd
	if cmd.Immediate {
		return s.iscsiExecTask(task)
	}
	cmdsn := cmd.CmdSN
	log.Debugf("CmdSN of command is %d", cmdsn)
	if cmdsn == sess.ExpCmdSN {
	retry:
		cmdsn += 1
		sess.ExpCmdSN = cmdsn
		log.Debugf("session's ExpCmdSN is %d", cmdsn)

		log.Debugf("process task(%d)", task.cmd.CmdSN)
		if err := s.iscsiExecTask(task); err != nil {
			log.Error(err)
		}
		if len(sess.PendingTasks) == 0 {
			return nil
		}
		sess.PendingTasksMutex.Lock()
		task = sess.PendingTasks.Pop().(*iscsiTask)
		cmd = task.cmd
		if cmd.CmdSN != cmdsn {
			sess.PendingTasks.Push(task)
			sess.PendingTasksMutex.Unlock()
			return nil
		}
		task.state = taskSCSI
		sess.PendingTasksMutex.Unlock()
		goto retry
	} else {
		if cmd.CmdSN < sess.ExpCmdSN {
			err := fmt.Errorf("unexpected cmd serial number: (%d, %d)", cmd.CmdSN, sess.ExpCmdSN)
			log.Error(err)
			return err
		}
		log.Debugf("add task(%d) into task queue", task.cmd.CmdSN)
		// add this task into queue and set it as a pending task
		sess.PendingTasksMutex.Lock()
		task.state = taskPending
		sess.PendingTasks.Push(task)
		sess.PendingTasksMutex.Unlock()
		return fmt.Errorf("pending")
	}

	return nil
}

func (s *ISCSITargetDriver) iscsiExecTask(task *iscsiTask) error {
	cmd := task.cmd
	switch cmd.OpCode {
	case OpSCSICmd, OpSCSIOut:
		task.state = taskSCSI
		// add scsi target process queue
		err := s.SCSI.AddCommandQueue(task.conn.session.Target.SCSITarget.TID, task.scmd)
		if err != nil {
			task.state = 0
		}
		return err
	case OpLogoutReq:

	case OpNoopOut:
		// just do it in iscsi layer
	case OpSCSITaskReq:
		sess := task.conn.session
		switch cmd.TaskFunc {
		case ISCSI_TM_FUNC_ABORT_TASK:
			var stask *iscsiTask
			sess.PendingTasksMutex.Lock()
			for i, t := range sess.PendingTasks {
				if cmd.ReferencedTaskTag == t.tag {
					stask = sess.PendingTasks[i]
					sess.PendingTasks = append(sess.PendingTasks[:i], sess.PendingTasks[i+1:]...)
					break
				}
			}
			sess.PendingTasksMutex.Unlock()
			if stask == nil {
				task.result = ISCSI_TMF_RSP_NO_TASK
			} else {
				// abort this task
				log.Debugf("abort the task[%v]", stask.tag)
				if stask.scmd == nil {
					stask.scmd = &api.SCSICommand{Result: api.SAM_STAT_TASK_ABORTED}
				}
				stask.conn = task.conn
				log.Debugf("stask.conn: %#v", stask.conn)
				stask.conn.buildRespPackage(OpSCSIResp, stask)
				stask.conn.rxTask = nil
				s.handler(DATAOUT, stask.conn)
				task.result = ISCSI_TMF_RSP_COMPLETE
			}
		case ISCSI_TM_FUNC_ABORT_TASK_SET:
		case ISCSI_TM_FUNC_LOGICAL_UNIT_RESET:
		case ISCSI_TM_FUNC_CLEAR_ACA:
			fallthrough
		case ISCSI_TM_FUNC_CLEAR_TASK_SET:
			fallthrough
		case ISCSI_TM_FUNC_TARGET_WARM_RESET, ISCSI_TM_FUNC_TARGET_COLD_RESET, ISCSI_TM_FUNC_TASK_REASSIGN:
			task.result = ISCSI_TMF_RSP_NOT_SUPPORTED
			//return fmt.Errorf("The task function is not supported")
		default:
			task.result = ISCSI_TMF_RSP_REJECTED
			//return fmt.Errorf("Unknown task function")
		}
		// return response to initiator
		return task.conn.buildRespPackage(OpSCSITaskResp, task)
	}
	return nil
}
