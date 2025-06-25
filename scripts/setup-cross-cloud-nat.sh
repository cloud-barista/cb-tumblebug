#!/bin/bash

# Ensure clean exit on any termination
trap 'exit 0' EXIT TERM INT

# Redirect any hanging file descriptors
exec 3>&- 4>&- 5>&- 6>&- 7>&- 8>&- 9>&-

set -e

# Initialize variables
PUBLIC_IPS=""
PRIVATE_IPS=""

# Parse command line arguments
for arg in "$@"; do
    case $arg in
        pub=*)
            PUBLIC_IPS="${arg#pub=}"
            ;;
        priv=*)
            PRIVATE_IPS="${arg#priv=}"
            ;;
        --help|-h)
            echo "Usage: $0 pub=<public_ip_list> priv=<private_ip_list>"
            echo
            echo "Parameters:"
            echo "  pub=   Comma-separated list of public IPs"
            echo "  priv=  Comma-separated list of private IPs (matching order)"
            echo
            echo "Examples:"
            echo "  $0 pub=52.231.1.1,13.125.2.2 priv=10.0.1.10,10.0.1.11"
            echo
            echo "Via curl:"
            echo "  curl -sSL <url> | sudo bash -s -- pub=1.1.1.1,2.2.2.2 priv=10.0.1.10,10.0.1.11"
            exit 0
            ;;
        *)
            echo "Unknown parameter: $arg"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Validate required parameters
if [ -z "$PUBLIC_IPS" ] || [ -z "$PRIVATE_IPS" ]; then
    echo "Error: Both 'pub' and 'priv' parameters are required"
    echo
    echo "Usage: $0 pub=<public_ip_list> priv=<private_ip_list>"
    echo "Example: $0 pub=52.231.1.1,13.125.2.2 priv=10.0.1.10,10.0.1.11"
    exit 1
fi

# Convert comma-separated lists to arrays
IFS=',' read -ra PUBLIC_IP_ARRAY <<< "$PUBLIC_IPS"
IFS=',' read -ra PRIVATE_IP_ARRAY <<< "$PRIVATE_IPS"

# Validate array lengths match
if [ ${#PUBLIC_IP_ARRAY[@]} -ne ${#PRIVATE_IP_ARRAY[@]} ]; then
    echo "Error: Number of public IPs (${#PUBLIC_IP_ARRAY[@]}) doesn't match number of private IPs (${#PRIVATE_IP_ARRAY[@]})"
    exit 1
fi

# Function to detect current VM's index
detect_current_vm_index() {
    # Get all IPs assigned to this machine
    local all_ips=$(ip addr show | grep -oP '(?<=inet\s)\d+(\.\d+){3}' | grep -v '127.0.0.1')
    
    # Check private IPs first
    for i in "${!PRIVATE_IP_ARRAY[@]}"; do
        if echo "$all_ips" | grep -q "^${PRIVATE_IP_ARRAY[$i]}$"; then
            echo $i
            return 0
        fi
    done
    
    # Check public IPs
    for i in "${!PUBLIC_IP_ARRAY[@]}"; do
        if echo "$all_ips" | grep -q "^${PUBLIC_IP_ARRAY[$i]}$"; then
            echo $i
            return 0
        fi
    done
    
    echo -1
    return 1
}

# Function to check if iptables rule exists
rule_exists() {
    local table=$1
    local chain=$2
    shift 2
    iptables -w -t "$table" -C "$chain" "$@" 2>/dev/null
}

# Function to add or update iptables rule
add_or_update_iptables_rule() {
    local table=$1
    local chain=$2
    local private_ip=$3
    local public_ip=$4
    
    # Check if a rule for this private IP already exists
    local existing_rule=$(iptables -w -t "$table" -L "$chain" -n | grep "DNAT.*$private_ip" | head -1)
    
    if [ -n "$existing_rule" ]; then
        # Extract the current destination
        local current_dest=$(echo "$existing_rule" | grep -oP 'to:\K[0-9.]+')
        
        if [ "$current_dest" != "$public_ip" ]; then
            # Remove old rule
            iptables -w -t "$table" -D "$chain" -d "$private_ip" -j DNAT --to-destination "$current_dest" 2>/dev/null || true
            # Add new rule
            iptables -w -t "$table" -A "$chain" -d "$private_ip" -j DNAT --to-destination "$public_ip"
            echo "Updated rule: $private_ip -> $public_ip (was -> $current_dest)"
        else
            echo "Rule already exists with same destination: $private_ip -> $public_ip"
        fi
    else
        # Add new rule
        iptables -w -t "$table" -A "$chain" -d "$private_ip" -j DNAT --to-destination "$public_ip"
        echo "Added rule: $private_ip -> $public_ip"
    fi
}

# Function to check if IP is reachable (same network)
is_ip_reachable() {
    local ip=$1
    # Try arping first (layer 2, more reliable for same network detection)
    if command -v arping &>/dev/null; then
        arping -c 1 -w 1 "$ip" &>/dev/null 2>&1 && return 0
    fi
    # Fallback to ping with strict timeout
    timeout 1 ping -c 1 -W 1 "$ip" &>/dev/null 2>&1
}

# Main setup function
setup_nat_rules() {
    echo "=== Starting NAT Setup on $(hostname) ==="
    echo "Public IPs: ${PUBLIC_IP_ARRAY[*]}"
    echo "Private IPs: ${PRIVATE_IP_ARRAY[*]}"
    
    # Detect current VM
    local current_index=$(detect_current_vm_index)
    if [ $current_index -eq -1 ]; then
        echo "ERROR: Could not detect current VM from the provided IP lists"
        echo "Make sure this VM's IP is included in either the public or private IP list"
        echo "Current VM IPs:"
        ip addr show | grep -oP '(?<=inet\s)\d+(\.\d+){3}' | grep -v '127.0.0.1'
        exit 1
    else
        echo "Detected current VM index: $current_index"
        echo "Current Private IP: ${PRIVATE_IP_ARRAY[$current_index]}"
        echo "Current Public IP: ${PUBLIC_IP_ARRAY[$current_index]}"
    fi
    
    # Setup NAT rules for all other VMs
    for i in "${!PRIVATE_IP_ARRAY[@]}"; do
        # Skip current VM
        if [ $i -eq $current_index ]; then
            echo "Skipping current VM (index $i)"
            continue
        fi
        
        local private_ip="${PRIVATE_IP_ARRAY[$i]}"
        local public_ip="${PUBLIC_IP_ARRAY[$i]}"
        
        # Check if this VM is directly reachable via private IP
        echo -n "Checking connectivity to $private_ip... "
        if is_ip_reachable "$private_ip"; then
            echo "REACHABLE (same network) - skipping NAT rule"
            continue
        else
            echo "NOT REACHABLE - adding NAT rule"
        fi
        
        echo "Setting up NAT: $private_ip -> $public_ip"
        
        # OUTPUT chain - for locally generated packets
        add_or_update_iptables_rule nat OUTPUT "$private_ip" "$public_ip"
        
        # PREROUTING chain - for forwarded packets
        add_or_update_iptables_rule nat PREROUTING "$private_ip" "$public_ip"
    done
    
    # Clean up orphaned rules (private IPs not in current list)
    echo "Cleaning up orphaned NAT rules..."
    
    # Store rules to remove after iteration
    local rules_to_remove=()
    
    for chain in OUTPUT PREROUTING; do
        while IFS= read -r line; do
            if [[ "$line" =~ DNAT ]]; then
                local rule_ip=$(echo "$line" | awk '{print $5}')
                local found=0
                
                # Check if this IP is in our current private IP list
                for private_ip in "${PRIVATE_IP_ARRAY[@]}"; do
                    if [ "$rule_ip" = "$private_ip" ]; then
                        found=1
                        break
                    fi
                done
                
                # If not found, add to removal list
                if [ $found -eq 0 ] && [ -n "$rule_ip" ]; then
                    local dest=$(echo "$line" | grep -oP 'to:\K[0-9.]+')
                    if [ -n "$dest" ]; then
                        rules_to_remove+=("$chain:$rule_ip:$dest")
                    fi
                fi
            fi
        done < <(iptables -w -t nat -L "$chain" -n 2>/dev/null)
    done
    
    # Remove orphaned rules
    for rule in "${rules_to_remove[@]}"; do
        IFS=':' read -r chain rule_ip dest <<< "$rule"
        iptables -w -t nat -D "$chain" -d "$rule_ip" -j DNAT --to-destination "$dest" 2>/dev/null && \
            echo "Removed orphaned rule: $rule_ip -> $dest"
    done
    
    # Enable IP forwarding
    local current_forward=$(sysctl -n net.ipv4.ip_forward)
    if [ "$current_forward" -eq 0 ]; then
        sysctl -w net.ipv4.ip_forward=1
        echo "Enabled IP forwarding"
        
        # Make it persistent
        if ! grep -q "net.ipv4.ip_forward=1" /etc/sysctl.conf; then
            echo "net.ipv4.ip_forward=1" >> /etc/sysctl.conf
            echo "Made IP forwarding persistent"
        fi
    else
        echo "IP forwarding already enabled"
    fi
    
    # Save iptables rules persistently
    echo "Saving iptables rules..."
    
    # Skip save if running in container or minimal environment
    if [ -f /.dockerenv ] || [ -f /run/systemd/container ]; then
        echo "Container environment detected, skipping persistent save"
    else
        # Try different methods based on the distribution
        if command -v netfilter-persistent &>/dev/null 2>&1; then
            timeout 5 netfilter-persistent save </dev/null &>/dev/null 2>&1 || echo "Warning: netfilter-persistent save failed"
        elif [ -f /etc/debian_version ]; then
            # Debian/Ubuntu
            if ! command -v iptables-save &>/dev/null 2>&1; then
                echo "Installing iptables-persistent..."
                DEBIAN_FRONTEND=noninteractive apt-get update </dev/null &>/dev/null 2>&1
                DEBIAN_FRONTEND=noninteractive apt-get install -y iptables-persistent </dev/null &>/dev/null 2>&1
            fi
            mkdir -p /etc/iptables 2>/dev/null || true
            iptables-save > /etc/iptables/rules.v4 2>/dev/null || echo "Warning: Could not save to /etc/iptables/rules.v4"
        elif [ -f /etc/redhat-release ]; then
            # RHEL/CentOS
            service iptables save </dev/null &>/dev/null 2>&1 || \
            iptables-save > /etc/sysconfig/iptables 2>/dev/null || true
        else
            # Generic fallback - just skip
            echo "Unknown distribution, skipping persistent save"
        fi
    fi
    
    echo "=== NAT Setup Complete ==="
}

# Function to display current NAT rules
show_nat_rules() {
    echo
    echo "=== Current NAT Rules ==="
    echo "OUTPUT chain:"
    iptables -w -t nat -L OUTPUT -n -v | grep -E "DNAT|Chain OUTPUT" || true
    echo
    echo "PREROUTING chain:"
    iptables -w -t nat -L PREROUTING -n -v | grep -E "DNAT|Chain PREROUTING" || true
    echo
}

# Function to test connectivity
test_connectivity() {
    echo
    echo "=== Testing Connectivity ==="
    
    local current_index=$(detect_current_vm_index)
    if [ $current_index -eq -1 ]; then
        echo "Could not detect current VM for testing"
        return
    fi
    
    for i in "${!PRIVATE_IP_ARRAY[@]}"; do
        # Skip current VM
        if [ $i -eq $current_index ]; then
            continue
        fi
        
        local private_ip="${PRIVATE_IP_ARRAY[$i]}"
        local public_ip="${PUBLIC_IP_ARRAY[$i]}"
        
        echo
        echo "Testing VM $i:"
        echo "  Private IP: $private_ip"
        echo "  Public IP: $public_ip"
        
        # Test ping to private IP (should be routed to public IP)
        echo -n "  Ping test (via private IP): "
        if timeout 2 ping -c 1 -W 2 "$private_ip" &>/dev/null; then
            echo "SUCCESS"
        else
            echo "FAILED"
        fi
    done
    echo
}

# Execute main functions with clean stdio
{
    setup_nat_rules
    show_nat_rules
    test_connectivity
} </dev/null 2>&1 | cat

# Ensure all file descriptors are closed
exec 0<&- 1>&- 2>&-

# Kill any remaining background processes from this script
pkill -P $ 2>/dev/null || true

# Force exit
exit 0
