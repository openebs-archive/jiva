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

from GUIConfig import GuiConfig as const
import docker,time,subprocess
from subprocess import Popen,STDOUT,PIPE
client = docker.DockerClient(version="auto")
out = client.containers.list(all)
if len(out) !=0:
	for container in client.containers.list(all):
       		print "Check status,then stop and remove Container with ID: ",container.id
	        status = container.status
		if status == "running":
       			container.stop()
			container.remove()
		elif status == "exited":
			container.remove()
else:
	print "There are no containers"

#Assigning ip to interface#
ip1 = "ip addr delete %s/24 dev eth1" % const.Controller_IP
ip2 = "ip addr delete %s/24 dev eth2" % const.Replica_IP
sp = subprocess.Popen(ip1, shell=True, stdin=PIPE, stdout=PIPE, stderr=STDOUT, close_fds=True)
ip1 = sp.stdout.read()
output, errors = sp.communicate()
print ip1
sp1 = subprocess.Popen(ip2, shell=True, stdin=PIPE, stdout=PIPE, stderr=STDOUT, close_fds=True)
ip2 = sp1.stdout.read()
output, errors = sp1.communicate()
print ip2

