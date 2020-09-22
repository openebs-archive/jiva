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

set -o history -o histexpand
should_exit(){
	if [ $1 -ne 0 ];
	then
	        echo "Configuration failed Exit Status `history | tail -n 3 | awk 'NR==2 { print; exit }'`"
	        exit
	fi
}
python JIVA_Create.py
should_exit $?

python JIVA_libiscsi.py
should_exit $?


