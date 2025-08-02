-- Fix audit trigger to generate ID

BEGIN;

-- Set search path to mcp schema
SET search_path TO mcp, public;

CREATE OR REPLACE FUNCTION audit_trigger_function()
RETURNS TRIGGER AS $$
DECLARE
    audit_row audit_log;
    changed_fields TEXT[];
BEGIN
    audit_row.id := uuid_generate_v4(); -- Generate ID
    audit_row.tenant_id := COALESCE(NEW.tenant_id, OLD.tenant_id);
    audit_row.table_name := TG_TABLE_NAME;
    audit_row.action := TG_OP;
    audit_row.changed_by := COALESCE(current_setting('app.current_user', true), 'system');
    audit_row.ip_address := COALESCE(inet(current_setting('app.client_ip', true)), NULL);
    audit_row.user_agent := current_setting('app.user_agent', true);
    audit_row.changed_at := CURRENT_TIMESTAMP;
    
    IF TG_OP = 'DELETE' THEN
        audit_row.record_id := OLD.id;
        audit_row.old_data := to_jsonb(OLD);
    ELSIF TG_OP = 'UPDATE' THEN
        audit_row.record_id := NEW.id;
        audit_row.old_data := to_jsonb(OLD);
        audit_row.new_data := to_jsonb(NEW);
        
        -- Calculate changed fields
        SELECT array_agg(key) INTO changed_fields
        FROM jsonb_each(to_jsonb(NEW))
        WHERE to_jsonb(NEW) -> key IS DISTINCT FROM to_jsonb(OLD) -> key;
        
        audit_row.changed_fields := changed_fields;
    ELSIF TG_OP = 'INSERT' THEN
        audit_row.record_id := NEW.id;
        audit_row.new_data := to_jsonb(NEW);
    END IF;
    
    INSERT INTO audit_log VALUES (audit_row.*);
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

COMMIT;