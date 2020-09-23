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

networkID=br-`sudo docker network list | grep stg-net | awk '{print $1}'`
sudo ip addr add 172.18.0.110 dev $networkID
sudo ip addr add 172.18.0.111 dev $networkID
sudo ip addr add 172.18.0.112 dev $networkID
sudo ip addr add 172.18.0.113 dev $networkID
env REPLICATION_FACTOR=3 ./functional
