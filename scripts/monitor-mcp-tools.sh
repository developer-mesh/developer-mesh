#!/bin/bash

# MCP Tool Usage Monitor for Claude Code
# This script monitors Claude Code's tool usage to ensure MCP tools are prioritized

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m' # No Color

# Log file locations
CLAUDE_LOG="${CLAUDE_LOG:-$HOME/.claude/logs/latest.log}"
EDGE_MCP_LOG="${EDGE_MCP_LOG:-/tmp/edge-mcp.log}"

echo -e "${PURPLE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${PURPLE}   MCP Tool Usage Monitor for Claude Code${NC}"
echo -e "${PURPLE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Function to check if MCP is running
check_mcp_status() {
    if pgrep -f "edge-mcp" > /dev/null; then
        echo -e "${GREEN}✓${NC} Edge MCP is running"
        return 0
    else
        echo -e "${RED}✗${NC} Edge MCP is not running"
        echo -e "${YELLOW}  Start it with: edge-mcp --stdio${NC}"
        return 1
    fi
}

# Function to monitor tool usage in real-time
monitor_tools() {
    echo -e "\n${BLUE}Monitoring tool usage (press Ctrl+C to stop)...${NC}\n"
    
    # Create a temporary file for tracking stats
    STATS_FILE="/tmp/mcp-tool-stats-$$.txt"
    echo "mcp:0" > "$STATS_FILE"
    echo "builtin:0" >> "$STATS_FILE"
    
    # Monitor logs (adjust based on your Claude log format)
    tail -f "$CLAUDE_LOG" 2>/dev/null | while read -r line; do
        # Check for MCP tool usage
        if echo "$line" | grep -qE "(mcp__|devmesh_|tools/call.*mcp)"; then
            TOOL=$(echo "$line" | grep -oE "mcp__[a-zA-Z_]*|devmesh__[a-zA-Z_]*" | head -1)
            if [ -n "$TOOL" ]; then
                echo -e "${GREEN}[$(date '+%H:%M:%S')] ✓ MCP Tool Used:${NC} $TOOL"
                # Increment MCP counter
                MCP_COUNT=$(grep "^mcp:" "$STATS_FILE" | cut -d: -f2)
                MCP_COUNT=$((MCP_COUNT + 1))
                sed -i.bak "s/^mcp:.*/mcp:$MCP_COUNT/" "$STATS_FILE"
            fi
        fi
        
        # Check for built-in tool usage (Read, Write, Bash, etc.)
        if echo "$line" | grep -qE "tool.*\"(Read|Write|Bash|Edit|MultiEdit|Grep|Glob)\"" && \
           ! echo "$line" | grep -qE "(mcp__|devmesh_)"; then
            TOOL=$(echo "$line" | grep -oE "\"(Read|Write|Bash|Edit|MultiEdit|Grep|Glob)\"" | tr -d '"' | head -1)
            if [ -n "$TOOL" ]; then
                echo -e "${YELLOW}[$(date '+%H:%M:%S')] ⚠ Built-in Tool:${NC} $TOOL"
                # Increment built-in counter
                BUILTIN_COUNT=$(grep "^builtin:" "$STATS_FILE" | cut -d: -f2)
                BUILTIN_COUNT=$((BUILTIN_COUNT + 1))
                sed -i.bak "s/^builtin:.*/builtin:$BUILTIN_COUNT/" "$STATS_FILE"
            fi
        fi
        
        # Check for tools/list calls (tool discovery)
        if echo "$line" | grep -q "tools/list"; then
            echo -e "${BLUE}[$(date '+%H:%M:%S')] 🔍 Tool Discovery:${NC} Querying available tools"
        fi
        
        # Periodically show stats
        if [ $((RANDOM % 20)) -eq 0 ]; then
            MCP_COUNT=$(grep "^mcp:" "$STATS_FILE" | cut -d: -f2)
            BUILTIN_COUNT=$(grep "^builtin:" "$STATS_FILE" | cut -d: -f2)
            if [ "$MCP_COUNT" -gt 0 ] || [ "$BUILTIN_COUNT" -gt 0 ]; then
                echo -e "\n${PURPLE}──── Statistics ────${NC}"
                echo -e "MCP Tools: ${GREEN}$MCP_COUNT${NC} | Built-in: ${YELLOW}$BUILTIN_COUNT${NC}"
                if [ "$MCP_COUNT" -gt 0 ] && [ "$BUILTIN_COUNT" -gt 0 ]; then
                    RATIO=$(echo "scale=1; $MCP_COUNT * 100 / ($MCP_COUNT + $BUILTIN_COUNT)" | bc)
                    echo -e "MCP Usage: ${GREEN}${RATIO}%${NC}"
                fi
                echo -e "${PURPLE}────────────────────${NC}\n"
            fi
        fi
    done
}

# Function to test MCP tool availability
test_mcp_tools() {
    echo -e "\n${BLUE}Testing MCP tool availability...${NC}\n"
    
    # Check if we can connect to the MCP server
    if command -v websocat >/dev/null 2>&1; then
        echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | \
            websocat -n1 ws://localhost:8082/ws 2>/dev/null | \
            python3 -m json.tool 2>/dev/null | \
            grep -E '"name"|"description"' | \
            head -20
    else
        echo -e "${YELLOW}websocat not installed. Install with: brew install websocat${NC}"
    fi
}

# Function to show configuration status
show_config_status() {
    echo -e "\n${BLUE}Configuration Status:${NC}\n"
    
    # Check global CLAUDE.md
    if [ -f "$HOME/.claude/CLAUDE.md" ]; then
        if grep -q "MCP Tool Prioritization" "$HOME/.claude/CLAUDE.md"; then
            echo -e "${GREEN}✓${NC} Global CLAUDE.md has MCP prioritization rules"
        else
            echo -e "${YELLOW}⚠${NC} Global CLAUDE.md missing MCP prioritization rules"
        fi
    else
        echo -e "${RED}✗${NC} Global CLAUDE.md not found at ~/.claude/CLAUDE.md"
    fi
    
    # Check for MCP in settings
    if [ -f "$HOME/.claude/settings.json" ]; then
        if grep -q "mcp__" "$HOME/.claude/settings.json"; then
            echo -e "${GREEN}✓${NC} MCP tools found in Claude settings"
        else
            echo -e "${YELLOW}⚠${NC} No MCP tools in Claude settings"
        fi
    fi
    
    # Check for local project settings
    if [ -f ".claude/settings.local.json" ]; then
        echo -e "${GREEN}✓${NC} Project-specific Claude settings found"
    fi
    
    # Check environment variables
    if [ -n "$DEV_MESH_URL" ]; then
        echo -e "${GREEN}✓${NC} DEV_MESH_URL is set: $DEV_MESH_URL"
    else
        echo -e "${YELLOW}⚠${NC} DEV_MESH_URL not set"
    fi
    
    if [ -n "$DEV_MESH_API_KEY" ]; then
        echo -e "${GREEN}✓${NC} DEV_MESH_API_KEY is set"
    else
        echo -e "${YELLOW}⚠${NC} DEV_MESH_API_KEY not set"
    fi
}

# Main menu
main() {
    while true; do
        echo -e "\n${BLUE}Select an option:${NC}"
        echo "1) Check MCP status"
        echo "2) Monitor tool usage in real-time"
        echo "3) Test MCP tool availability"
        echo "4) Show configuration status"
        echo "5) View recent tool usage summary"
        echo "6) Exit"
        
        read -p "Choice: " choice
        
        case $choice in
            1)
                check_mcp_status
                ;;
            2)
                if check_mcp_status; then
                    monitor_tools
                fi
                ;;
            3)
                test_mcp_tools
                ;;
            4)
                show_config_status
                ;;
            5)
                if [ -f "$CLAUDE_LOG" ]; then
                    echo -e "\n${BLUE}Recent tool usage (last 50 lines):${NC}\n"
                    grep -E "(mcp__|devmesh_|tool.*\"(Read|Write|Bash|Edit)\")" "$CLAUDE_LOG" | tail -50 | while read -r line; do
                        if echo "$line" | grep -qE "(mcp__|devmesh_)"; then
                            echo -e "${GREEN}[MCP]${NC} $line"
                        else
                            echo -e "${YELLOW}[Built-in]${NC} $line"
                        fi
                    done
                else
                    echo -e "${RED}Claude log not found at $CLAUDE_LOG${NC}"
                fi
                ;;
            6)
                echo -e "${PURPLE}Goodbye!${NC}"
                exit 0
                ;;
            *)
                echo -e "${RED}Invalid choice${NC}"
                ;;
        esac
    done
}

# Handle cleanup on exit
trap 'rm -f /tmp/mcp-tool-stats-*.txt' EXIT

# Run main menu
main