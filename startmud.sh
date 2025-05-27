
#export CONFIG_PATH=/GoMudLLM/GoMud/_datafiles/config.custom.yaml
# ./go-mud-server
     #!/bin/bash
     # Example: MUD server init script
     # ### BEGIN INIT INFO
     # Provides: mud-server
     # Required-Start: $local_fs $network $syslog
     # Required-Stop: $local_fs $network $syslog
     # Default-Start: 2 3 4 5
     # Default-Stop: 0 1 6
     # Short-Description: MUD Server
     # Description: Starts and stops the MUD server
     # ### END INIT INFO

     SCRIPTNAME="gosavsoul"
     MUDDIR="/GoMudLLM/GoMud/"
     SCRIPTDIR="/GoMudLLM/GoMud/startmud.sh" # Replace with the actual path
     SERVER_BINARY="/GoMudLLM/GoMud/go-mud-server" # Replace with the path to your server binary
     PIDFILE="/var/run/$SCRIPTNAME.pid"

     # Function to check if the server is running
     status() {
         if [ -f "$PIDFILE" ] && pgrep -f "$SERVER_BINARY" >/dev/null; then
             echo "MUD Server is running"
         else
             echo "MUD Server is not running"
         fi
     }

     # Function to start the server
     start() {
         echo "Starting MUD Server..."
         if [ ! -f "$PIDFILE" ]; then
             mkdir -p $(dirname "$PIDFILE")
	     cd "$MUDDIR"
	     echo "starting binary....."
             export CONFIG_PATH=/GoMudLLM/GoMud/_datafiles/config.custom.yaml
             "$SERVER_BINARY" >/dev/null 2>&1 &
             echo $! > "$PIDFILE"
             echo "MUD Server started"
         else
             echo "MUD Server already running"
         fi
     }

     # Function to stop the server
     stop() {
         echo "Stopping MUD Server..."
         if [ -f "$PIDFILE" ]; then
             PID=$(cat "$PIDFILE")
             if pgrep -f "$SERVER_BINARY" >/dev/null; then
                 kill $PID
                 wait $PID
                 rm -f "$PIDFILE"
                 echo "MUD Server stopped"
             else
                 echo "MUD Server not running"
             fi
         else
             echo "MUD Server not running"
         fi
     }

     # Function to restart the server
     restart() {
         stop
         start
     }

     case "$1" in
         start)
             start
         ;;
         stop)
             stop
         ;;
         restart)
             restart
         ;;
         status)
             status
         ;;
         *)
             echo "Usage: $0 {start|stop|restart|status}"
             exit 1
         ;;
     esac

     exit 0
