
networkID=br-`sudo docker network list | grep stg-net | awk '{print $1}'`
sudo ip addr add 172.18.0.110 dev $networkID
sudo ip addr add 172.18.0.111 dev $networkID
sudo ip addr add 172.18.0.112 dev $networkID
sudo ip addr add 172.18.0.113 dev $networkID
env REPLICATION_FACTOR=3 ./functional
