sudo ip addr add 172.17.0.110 dev docker0
sudo ip addr add 172.17.0.111 dev docker0
sudo ip addr add 172.17.0.112 dev docker0
sudo ip addr add 172.17.0.113 dev docker0
env REPLICATION_FACTOR=3 ./functional
