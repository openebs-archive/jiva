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


