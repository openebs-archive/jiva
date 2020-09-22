# Copyright © 2020 The OpenEBS Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import docker,time
import subprocess
from subprocess import Popen,STDOUT,PIPE
from GUIConfig import GuiConfig as const
#Assigning ip to interface#
ip1 = "ip addr add %s/24 dev eth1" % const.Controller_IP
ip2 = "ip addr add %s/24 dev eth2" % const.Replica_IP
sp = subprocess.Popen(ip1, shell=True, stdin=PIPE, stdout=PIPE, stderr=STDOUT, close_fds=True)
ip1 = sp.stdout.read()
output, errors = sp.communicate()
print ip1
sp1 = subprocess.Popen(ip2, shell=True, stdin=PIPE, stdout=PIPE, stderr=STDOUT, close_fds=True)
ip2 = sp1.stdout.read()
output, errors = sp1.communicate()
print ip2
client = docker.DockerClient(version="auto")
client.containers.run("openebs/jiva",name=const.Controller_Name,detach=True,network_mode="host",ports={'3260/tcp': 9501},command="launch controller --frontend gotgt --frontendIP %s %s %s" % (const.Controller_IP, const.Volume_name, const.Volume_size))
time.sleep(2)
print client.containers.get(const.Controller_Name)
client.containers.run("openebs/jiva",detach=True,name=const.Replica_Name,network_mode="host",ports={'9502/tcp': [9700,9800]},volumes={'/mnt/stor1': {'bind': '/stor1', 'mode': 'rw'}},command="launch replica --frontendIP %s --listen %s:9502 --size %s /stor1" % (const.Controller_IP, const.Replica_IP, const.Volume_size))
time.sleep(2)
print client.containers.get(const.Replica_Name)
#client.containers.run("openebs/jiva",detach=True,name=const.Replica1_Name,network_mode="host",ports={'9502/tcp': [9700,9800]},volumes={'/mnt/stor2': {'bind': '/stor2', 'mode': 'rw'}},command="launch replica --frontendIP %s --listen %s:9502 --size %s /stor2" % (const.Controller_IP, const.Replica1_IP, const.Volume_size))
#print client.containers.get(const.Replica1_Name)
out = client.containers.list()
if len(out) !=0:
        for container in client.containers.list():
                print "Check status of container with ID: ",container.id
                status = container.status
                if status == "running":
			print "HealthCheck: PASS Container ID %s" % container.id
		else:
			print "HealthCheck: FAIL Container ID %s" % container.id
else:
	print "Error: No Containers created"
