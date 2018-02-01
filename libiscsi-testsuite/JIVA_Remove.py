#-------------------------------------------------------------------------------
# Name:        JIVA_Container.py
# Purpose:     Create JIVA container with one controller and one replica
# Author:      Sudarshan/Swarna
# Created:     10/01/2017
# Copyright:   (c) Cloudbyte 2017
# Licence:     <your licence>
#-------------------------------------------------------------------------------
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

