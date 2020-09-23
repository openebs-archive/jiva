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

import docker,time,re
import subprocess
from subprocess import Popen,STDOUT,PIPE
from GUIConfig import GuiConfig as const
client = docker.DockerClient(version="auto")
pull_image = 'ksatchit/libiscsi:latest'
images_list = client.images.list(pull_image)
if images_list:
	print "Image is already present"
	print "Starting Libiscsi TestSuite"
        client.containers.run("ksatchit/libiscsi",name="libiscsi",detach=True,network_mode="host",volumes={'/mnt/logs': {'bind': '/logs', 'mode': 'rw'}},command="./testiscsi.sh --ctrl-svc-ip %s" % const.Controller_IP)
	container = client.containers.get('libiscsi') 
	id = container.id[:12]
	cnt = client.containers.get(id)
	print cnt.logs()
	s = subprocess.Popen("rm -rf /root/JIVA/output.logs", shell=True, stdin=PIPE, stdout=PIPE, stderr=STDOUT, close_fds=True)
	s.communicate()
	command = "docker logs -f  %s >> output.logs" % id
	sp = subprocess.Popen(command, shell=True, stdin=PIPE, stdout=PIPE, stderr=STDOUT, close_fds=True)
	ip1 = sp.stdout.read()
	output, errors = sp.communicate()
	sp_status = sp.wait()
	print cnt.logs()
	print ip1
	if True:
        	cmd = "grep 'No of tests passed' '/root/JIVA/output.logs'"
        	sp1 = subprocess.Popen(cmd, shell=True, stdin=PIPE, stdout=PIPE, stderr=STDOUT, close_fds=True)
        	ip2 = sp1.stdout.read()
        	output, errors = sp1.communicate()
        	out = re.split(r'(\d+)', ip2)
        	print out[3]
        	if int(out[3]) <= 29:
                	print "libiscsi tests passed"
        	else:
                	print "libiscsi tests failed"
	else:
        	print "Failed to re-direct logs"

	
else:
	command = "docker image pull ksatchit/libiscsi:latest"
	sp = subprocess.Popen(command, shell=True, stdin=PIPE, stdout=PIPE, stderr=STDOUT, close_fds=True)
	ip1 = sp.stdout.read()
	output, errors = sp.communicate()
	print ip1
	images_list = client.images.list(pull_image)
	if images_list:
		print images_list
		print "Image %s Pull Success" % pull_image
		client.containers.run("ksatchit/libiscsi",detach=True,network_mode="host",volumes={'/mnt/logs': {'bind': '/logs', 'mode': 'rw'}},command="./testiscsi.sh --ctrl-svc-ip %s" % const.Controller_IP)
		container = client.containers.get('libiscsi')
        	id = container.id[:12]
        	cnt = client.containers.get(id)
        	print cnt.logs()
        	s = subprocess.Popen("rm -rf /root/JIVA/output.logs", shell=True, stdin=PIPE, stdout=PIPE, stderr=STDOUT, close_fds=True)
	        s.communicate()
        	command = "docker logs -f  %s >> output.logs" % id
	        sp = subprocess.Popen(command, shell=True, stdin=PIPE, stdout=PIPE, stderr=STDOUT, close_fds=True)
        	ip1 = sp.stdout.read()
	        output, errors = sp.communicate()
        	sp_status = sp.wait()
	        print ip1
        	if True:
                	cmd = "grep 'No of tests passed' '/root/JIVA/output.logs'"
	                sp1 = subprocess.Popen(cmd, shell=True, stdin=PIPE, stdout=PIPE, stderr=STDOUT, close_fds=True)
        	        ip2 = sp1.stdout.read()
                	output, errors = sp1.communicate()
	                out = re.split(r'(\d+)', ip2)
        	        print out[3]
                	if int(out[3]) <= 29:
                        	print "libiscsi tests passed"
	                else:
        	                print "libiscsi tests failed"
	        else:
        	        print "Failed to re-direct logs"

	else:
		print "Failed to pull image: Retry later" 


