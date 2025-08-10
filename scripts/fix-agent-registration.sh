#!/bin/bash

# Fix Agent Registration Issues - Immediate Workaround Script
# This script provides immediate fixes for agent registration issues

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${YELLOW}=== Agent Registration Fix Script ===${NC}"

# Database connection parameters
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-devmesh_development}"
DB_USER="${DB_USER:-devmesh}"
DB_PASSWORD="${DB_PASSWORD:-devmesh123}"

# Function to execute SQL
execute_sql() {
    local sql="$1"
    PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "$sql"
}

# Option 1: Clean up duplicate agents for local development
cleanup_local_agents() {
    echo -e "${YELLOW}Cleaning up local MCP client agents...${NC}"
    
    execute_sql "
        DELETE FROM mcp.agents 
        WHERE name LIKE 'Local MCP Client%' 
        AND tenant_id = '00000000-0000-0000-0000-000000000001'
        AND created_at < NOW() - INTERVAL '1 hour';
    "
    
    echo -e "${GREEN}✓ Cleaned up old local agent registrations${NC}"
}

# Option 2: Add database trigger for automatic handling
install_upsert_trigger() {
    echo -e "${YELLOW}Installing database trigger for idempotent registration...${NC}"
    
    execute_sql "
        -- Create function to handle agent upserts
        CREATE OR REPLACE FUNCTION mcp.handle_agent_upsert()
        RETURNS TRIGGER AS \$\$
        BEGIN
            -- Check if agent with same name and tenant exists
            IF EXISTS (
                SELECT 1 FROM mcp.agents 
                WHERE tenant_id = NEW.tenant_id 
                AND name = NEW.name 
                AND id != NEW.id
            ) THEN
                -- Update the existing agent instead of inserting
                UPDATE mcp.agents 
                SET 
                    id = NEW.id,
                    type = COALESCE(NEW.type, type),
                    model_id = COALESCE(NEW.model_id, model_id),
                    capabilities = COALESCE(NEW.capabilities, capabilities),
                    status = COALESCE(NEW.status, status),
                    configuration = COALESCE(NEW.configuration, configuration),
                    last_seen_at = NOW(),
                    updated_at = NOW()
                WHERE tenant_id = NEW.tenant_id AND name = NEW.name;
                
                -- Return NULL to skip the insert
                RETURN NULL;
            END IF;
            
            -- Allow normal insert for new agents
            RETURN NEW;
        END;
        \$\$ LANGUAGE plpgsql;

        -- Create trigger if not exists
        DROP TRIGGER IF EXISTS agent_upsert_trigger ON mcp.agents;
        CREATE TRIGGER agent_upsert_trigger
            BEFORE INSERT ON mcp.agents
            FOR EACH ROW
            EXECUTE FUNCTION mcp.handle_agent_upsert();
    "
    
    echo -e "${GREEN}✓ Database trigger installed for idempotent registration${NC}"
}

# Option 3: Modify unique constraint to be more flexible
modify_constraint() {
    echo -e "${YELLOW}Modifying unique constraint for better flexibility...${NC}"
    
    execute_sql "
        -- Drop the existing constraint
        ALTER TABLE mcp.agents 
        DROP CONSTRAINT IF EXISTS agents_tenant_id_name_key;
        
        -- Add new constraint that includes status
        -- This allows 'inactive' duplicates but only one 'active' agent
        CREATE UNIQUE INDEX agents_tenant_name_active_idx 
        ON mcp.agents(tenant_id, name) 
        WHERE status = 'active' OR status = 'available';
    "
    
    echo -e "${GREEN}✓ Modified constraint to allow multiple inactive agents${NC}"
}

# Option 4: Add helper stored procedure for registration
add_registration_procedure() {
    echo -e "${YELLOW}Adding stored procedure for smart registration...${NC}"
    
    execute_sql "
        CREATE OR REPLACE FUNCTION mcp.register_or_update_agent(
            p_id UUID,
            p_tenant_id UUID,
            p_name VARCHAR(255),
            p_type VARCHAR(100),
            p_model_id VARCHAR(255),
            p_capabilities TEXT[] DEFAULT NULL,
            p_status VARCHAR(50) DEFAULT 'available'
        )
        RETURNS TABLE(
            agent_id UUID,
            is_new BOOLEAN,
            message TEXT
        ) AS \$\$
        DECLARE
            v_existing_id UUID;
            v_is_new BOOLEAN := FALSE;
        BEGIN
            -- Check if agent exists by name and tenant
            SELECT id INTO v_existing_id
            FROM mcp.agents
            WHERE tenant_id = p_tenant_id AND name = p_name
            LIMIT 1;
            
            IF v_existing_id IS NOT NULL THEN
                -- Update existing agent
                UPDATE mcp.agents
                SET 
                    type = COALESCE(p_type, type),
                    model_id = COALESCE(p_model_id, model_id),
                    capabilities = COALESCE(p_capabilities, capabilities),
                    status = p_status,
                    last_seen_at = NOW(),
                    updated_at = NOW()
                WHERE id = v_existing_id;
                
                RETURN QUERY SELECT v_existing_id, FALSE, 'Agent updated successfully'::TEXT;
            ELSE
                -- Try to insert new agent
                BEGIN
                    INSERT INTO mcp.agents (
                        id, tenant_id, name, type, model_id, 
                        capabilities, status, created_at, updated_at
                    ) VALUES (
                        p_id, p_tenant_id, p_name, p_type, p_model_id,
                        p_capabilities, p_status, NOW(), NOW()
                    );
                    
                    v_is_new := TRUE;
                    RETURN QUERY SELECT p_id, TRUE, 'Agent created successfully'::TEXT;
                EXCEPTION
                    WHEN unique_violation THEN
                        -- Race condition - another process created it
                        SELECT id INTO v_existing_id
                        FROM mcp.agents
                        WHERE tenant_id = p_tenant_id AND name = p_name;
                        
                        RETURN QUERY SELECT v_existing_id, FALSE, 'Agent exists (race condition handled)'::TEXT;
                END;
            END IF;
        END;
        \$\$ LANGUAGE plpgsql;
    "
    
    echo -e "${GREEN}✓ Added smart registration stored procedure${NC}"
}

# Main menu
echo -e "\n${YELLOW}Select fix option:${NC}"
echo "1. Clean up old local agents (quick fix)"
echo "2. Install database trigger (automatic handling)"
echo "3. Modify unique constraint (flexible approach)"
echo "4. Add registration stored procedure (recommended)"
echo "5. Apply all fixes (comprehensive solution)"
echo "6. Exit"

read -p "Enter option (1-6): " option

case $option in
    1)
        cleanup_local_agents
        ;;
    2)
        install_upsert_trigger
        ;;
    3)
        modify_constraint
        ;;
    4)
        add_registration_procedure
        ;;
    5)
        echo -e "${YELLOW}Applying all fixes...${NC}"
        cleanup_local_agents
        modify_constraint
        add_registration_procedure
        # Skip trigger as it conflicts with stored procedure
        echo -e "${GREEN}✓ All fixes applied successfully${NC}"
        ;;
    6)
        echo "Exiting..."
        exit 0
        ;;
    *)
        echo -e "${RED}Invalid option${NC}"
        exit 1
        ;;
esac

echo -e "\n${GREEN}Fix applied successfully! You can now retry the agent registration.${NC}"
echo -e "${YELLOW}Note: For production, consider implementing these fixes in the application code.${NC}"