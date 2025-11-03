#!/bin/bash

# Check for help option
if [[ "$1" == "-h" ]] || [[ "$1" == "--help" ]]; then
    echo "CB-Tumblebug Initialization Script"
    echo ""
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "This script initializes CB-Tumblebug by registering credentials,"
    echo "loading assets (specs and images), and fetching price information."
    echo ""
    echo "Options:"
    echo "  -h, --help                    Show this help message"
    echo "  -y, --yes                     Automatically answer yes to prompts"
    echo "  --credentials, --credentials-only"
    echo "                                Register cloud credentials only"
    echo "  --load-assets, --load-assets-only"
    echo "                                Load common specs and images only"
    echo "  --fetch-price, --fetch-price-only"
    echo "                                Fetch price information only"
    echo ""
    echo "Examples:"
    echo "  $0                            # Run all steps (default)"
    echo "  $0 -y                         # Run all steps without confirmation"
    echo "  $0 --credentials-only         # Register credentials only"
    echo "  $0 --load-assets-only         # Load assets only"
    echo "  $0 --fetch-price-only         # Fetch price only"
    echo "  $0 --credentials --load-assets"
    echo "                                # Register credentials and load assets"
    echo ""
    echo "Environment Variables:"
    echo "  TUMBLEBUG_SERVER              Server address (default: localhost:1323)"
    echo "  TB_API_USERNAME               API username (default: default)"
    echo "  TB_API_PASSWORD               API password (default: default)"
    echo "  LOG                           Set to 'on' to enable resource monitoring"
    echo ""
    exit 0
fi

SCRIPT_DIR=$(cd $(dirname "$0") && pwd)

# Change to the script directory
pushd "$SCRIPT_DIR" > /dev/null

# Python version check
REQUIRED_VERSION="3.8.0"

PYTHON_VERSION=$(python3 --version | cut -d' ' -f2)
echo "Detected Python version: $PYTHON_VERSION"
PYTHON_MAJOR=$(echo $PYTHON_VERSION | cut -d. -f1)
PYTHON_MINOR=$(echo $PYTHON_VERSION | cut -d. -f2)
PYTHON_PATCH=$(echo $PYTHON_VERSION | cut -d. -f3)

# Check if the Python3 version is 3.8.0 or higher
REQUIRED_MAJOR=3
REQUIRED_MINOR=8
REQUIRED_PATCH=0

if [[ $PYTHON_MAJOR -gt $REQUIRED_MAJOR ]] || \
   [[ $PYTHON_MAJOR -eq $REQUIRED_MAJOR && $PYTHON_MINOR -gt $REQUIRED_MINOR ]] || \
   [[ $PYTHON_MAJOR -eq $REQUIRED_MAJOR && $PYTHON_MINOR -eq $REQUIRED_MINOR && $PYTHON_PATCH -ge $REQUIRED_PATCH ]]; then
    echo "Python version is sufficient."
else
    echo "This script requires Python $REQUIRED_MAJOR.$REQUIRED_MINOR.$REQUIRED_PATCH or higher. Please upgrade the version"
    echo "  * Upgrade command by uv: uv python install $REQUIRED_MAJOR.$REQUIRED_MINOR"
    exit 1
fi

# Ensure uv is installed
echo
echo "Checking for uv..."
if ! command -v uv &> /dev/null; then
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "uv is not installed"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "uv is an extremely fast Python package installer and resolver,"
    echo "designed as a drop-in replacement for pip and pip-tools."
    echo "It's required for this project to manage Python dependencies efficiently."
    echo
    echo "You can install it using one of these methods:"
    echo
    echo "Option 1: Direct install (recommended)"
    echo
    echo -e "\033[4;94mcurl -LsSf https://astral.sh/uv/install.sh | sh\033[0m"
    echo
    echo "Option 2: Visit the installation page"
    echo
    echo -e "\033[4;94mhttps://github.com/astral-sh/uv#installation\033[0m"
    echo
    echo
    echo "After installation, reload your shell environment with:"
    echo
    echo -e "\033[4;94msource ~/.bashrc\033[0m"
    echo
    echo "# or use source ~/.bash_profile or source ~/.profile depending on your system"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    exit 1
fi

    
# Record start time for elapsed seconds calculation
START_TIME=$(date +%s)

# Optional: monitor cb-tumblebug and cb-spider resource usage if STATS is set
if [[ "$LOG" == "on" ]]; then
    echo "Docker resource monitoring enabled (cb-tumblebug, cb-spider)"
    LOGFILE="docker_stats_$(date +'%Y%m%d_%H%M%S').csv"
    
    # Get total memory of the host in GiB (used for mem-total)
    TOTAL_MEM_GIB=$(free -g | awk '/^Mem:/{print $2}')

    
    # Updated header with elapsed seconds instead of timestamp
    echo "elapsed,tb-cpu,sp-cpu,sum-cpu,tb-mem,sp-mem,sum-mem,mem-unit,mem-total" > "$LOGFILE"
    
    # Start monitoring in background
    {
        while true; do
            # Calculate elapsed seconds from start
            CURRENT_TIME=$(date +%s)
            ELAPSED_SEC=$((CURRENT_TIME - START_TIME))
            
            # Get CPU usage without % and memory usage in bytes
            TB_CPU=$(docker stats cb-tumblebug --no-stream --format "{{.CPUPerc}}" | sed 's/%//g')
            SP_CPU=$(docker stats cb-spider --no-stream --format "{{.CPUPerc}}" | sed 's/%//g')
            
            # Get memory usage in bytes and convert to GiB
            TB_MEM_BYTES=$(docker stats cb-tumblebug --no-stream --format "{{.MemUsage}}" | awk '{print $1}')
            SP_MEM_BYTES=$(docker stats cb-spider --no-stream --format "{{.MemUsage}}" | awk '{print $1}')
            
            # Convert memory to GiB (handling MiB, GiB units)
            TB_MEM_GIB=$(echo $TB_MEM_BYTES | awk '{
                value=$1; 
                if (index(value, "MiB") > 0) {
                    sub("MiB", "", value); 
                    printf "%.2f", value/1024;
                } else if (index(value, "GiB") > 0) {
                    sub("GiB", "", value); 
                    printf "%.2f", value;
                }
            }')
            
            SP_MEM_GIB=$(echo $SP_MEM_BYTES | awk '{
                value=$1; 
                if (index(value, "MiB") > 0) {
                    sub("MiB", "", value); 
                    printf "%.2f", value/1024;
                } else if (index(value, "GiB") > 0) {
                    sub("GiB", "", value); 
                    printf "%.2f", value;
                }
            }')

            # Calculate totals using awk for better precision
            SUM_CPU=$(awk "BEGIN {printf \"%.2f\", $TB_CPU + $SP_CPU}")
            SUM_MEM=$(awk "BEGIN {printf \"%.2f\", $TB_MEM_GIB + $SP_MEM_GIB}")

            # Write data to log file with elapsed seconds and totals
            echo "$ELAPSED_SEC,$TB_CPU,$SP_CPU,$SUM_CPU,$TB_MEM_GIB,$SP_MEM_GIB,$SUM_MEM,GiB,$TOTAL_MEM_GIB" >> "$LOGFILE"
            sleep 1
        done
    } &
    MONITOR_PID=$!
fi

echo
echo "Running the application..."
uv run init.py "$@"

# Stop monitoring if it was started
if [[ ! -z "$MONITOR_PID" ]]; then
    kill "$MONITOR_PID" 2>/dev/null
    wait "$MONITOR_PID" 2>/dev/null
    echo "Docker stats log saved to $LOGFILE"
fi

# Elapsed time calculation in Minutes
END_TIME=$(date +%s)
ELAPSED_TIME=$((END_TIME - START_TIME))
ELAPSED_MINUTES=$((ELAPSED_TIME / 60))
echo "Elapsed time: $ELAPSED_MINUTES minutes"

echo
echo "Cleaning up the venv and uv.lock files..."
rm -rf .venv
rm -rf uv.lock # Make it commented out if you want to keep the lock file

echo
echo "Environment cleanup complete."

# Return to the original directory
popd > /dev/null
