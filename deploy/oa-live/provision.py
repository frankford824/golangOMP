#!/usr/bin/env python3
"""Create scoped service credentials after migration 152. Only run on the main ECS."""
from pathlib import Path
import os
import re
import secrets
import subprocess
import sys

directory=Path('/etc/yongbo-oa-live')
tables=['oa_office_templates','oa_office_template_versions','oa_office_delegates','oa_office_finance_actions','oa_office_documents','oa_office_files','oa_office_people','oa_office_notifications','oa_office_imports','oa_order_feedback','oa_order_task_links']
reader={
    'auth_permissions':'code,enabled',
    'users': 'id,display_name,mobile,employee_no,department_id,team_id,department,team,status,employment_type,is_config_super_admin',
    'tasks': 'id,task_no,product_name_snapshot,task_status,current_handler_id,designer_id',
    'user_roles':'user_id,role',
    'org_departments':'id,name,enabled', 'org_teams':'id,name,department_id,enabled',
    'auth_roles':'id,code,archived_at', 'auth_user_role_assignments':'id,user_id,role_id,scope_mode',
    'auth_role_permissions':'role_id,permission_code', 'auth_org_role_policies':'id,role_id,subject_type,subject_id,enabled,scope_mode',
    'auth_assignment_scope_subjects':'assignment_id,subject_id,subject_type',
}
def workflow_grants(sql,database,peer):
    version=sql("SELECT VERSION()")
    number=re.match(r'(\d+)\.(\d+)\.(\d+)',version)
    if not number or tuple(map(int,number.groups())) < (8,0,22):
        raise RuntimeError('OA append-only task auditing requires MySQL 8.0.22 or later')
    source=f"'oa_formal_reader'@'{peer}'"
    writer=f"'oa_formal_extension'@'{peer}'"
    for principal in ('oa_formal_reader','oa_formal_extension'):
        if sql(f"SELECT COUNT(*) FROM mysql.user WHERE user='{principal}' AND host='{peer}'")!='1':
            raise RuntimeError('Existing OA service account expected')
    sql(f"GRANT SELECT (mobile) ON `{database}`.`users` TO {source}")
    grants={
        'tasks':'SELECT (id, task_status), UPDATE (task_status)',
        'task_event_logs':'SELECT (task_id, sequence), INSERT (id, task_id, sequence, event_type, operator_id, payload, created_at)',
        'task_event_sequences':'SELECT (task_id, last_sequence), INSERT (task_id, last_sequence), UPDATE (task_id, last_sequence)',
    }
    for table,privileges in grants.items():
        sql(f"GRANT {privileges} ON `{database}`.`{table}` TO {writer}")

def office_grants(sql, database, peer):
    writer = f"'oa_formal_extension'@'{peer}'"
    if sql(f"SELECT COUNT(*) FROM mysql.user WHERE user='oa_formal_extension' AND host='{peer}'") != '1':
        raise RuntimeError('Existing OA extension account expected')
    applied = sql("SELECT COUNT(*) FROM schema_migrations WHERE file_name IN ('154_oa_finance_document_events.sql','155_oa_person_profiles.sql') AND status IN ('applied','baseline')")
    if applied != '2':
        raise RuntimeError('OA migrations 154 and 155 must be applied first')
    grants = {
        'oa_office_document_events': 'SELECT, INSERT',
        'oa_office_person_profiles': 'SELECT, INSERT, UPDATE',
        'oa_office_org_managers': 'SELECT, INSERT, UPDATE',
        'users': 'SELECT (id,display_name,employee_no,status,department_id,team_id,department,team,employment_type,is_config_super_admin)',
        'user_roles': 'SELECT (user_id,role)',
        'org_departments': 'SELECT (id,name,enabled)',
        'org_teams': 'SELECT (id,name,enabled,department_id)',
        'auth_roles': 'SELECT (id,code,archived_at)',
        'auth_user_role_assignments': 'SELECT (user_id,role_id)',
        'auth_org_role_policies': 'SELECT (role_id,subject_type,subject_id,enabled)',
    }
    for table, privileges in grants.items():
        sql(f"GRANT {privileges} ON `{database}`.`{table}` TO {writer}")

def main():
    os.umask(0o077)
    database=os.environ['DB_NAME']
    if not re.fullmatch(r'[a-zA-Z0-9_]+',database) or database=='yongbo_oa':raise RuntimeError('Expected the official core database')
    dbhost=os.environ['DB_HOST'];port=os.environ.get('DB_PORT') or '3306'
    mysql=['mysql','--no-defaults','--protocol=tcp','-h',dbhost,'-P',port,'-u',os.environ['DB_USER'],'-Ns',database]
    def sql(statement):
        r=subprocess.run(mysql,input=statement,text=True,capture_output=True,env={**os.environ,'MYSQL_PWD':os.environ['DB_PASS']})
        if r.returncode:raise RuntimeError('Provisioning query failed: '+r.stderr.partition(':')[0])
        return r.stdout.strip()
    peer=sql("SELECT SUBSTRING_INDEX(USER(),'@',-1)")
    if not re.fullmatch(r'[a-zA-Z0-9.:-]+',peer):raise RuntimeError('unexpected MySQL peer')
    registered=sql("SELECT COUNT(*) FROM schema_migrations WHERE file_name='152_oa_business_extensions.sql' AND status='applied'")
    if registered!='1':raise RuntimeError('Migration 152 must be applied first')
    if sys.argv[1:]==['--office-grants']:
        office_grants(sql,database,peer)
        print('OA office privileges applied.')
        return
    if sys.argv[1:]==['--workflow-grants']:
        workflow_grants(sql,database,peer)
        print('OA workflow privileges applied.')
        return
    if sys.argv[1:]:
        raise RuntimeError('Unsupported invocation')
    directory.mkdir(mode=0o700,exist_ok=True)
    params={}
    for name,user in [('OA_SOURCE_DSN','oa_formal_reader'),('OA_EXTENSION_DSN','oa_formal_extension')]:
        file=directory/(user+'.password')
        existing=sql(f"SELECT COUNT(*) FROM mysql.user WHERE user='{user}' AND host='{peer}'")
        if not file.exists():
            if existing!='0':raise RuntimeError('Refusing to adopt an existing database account')
            file.write_text(secrets.token_hex(32))
        password=file.read_text().strip()
        if existing=='0':sql(f"CREATE USER '{user}'@'{peer}' IDENTIFIED BY '{password}'")
        if user=='oa_formal_reader':
            for tab,cols in reader.items():sql(f"GRANT SELECT ({cols}) ON `{database}`.`{tab}` TO '{user}'@'{peer}'")
        else:
            for tab in tables:sql(f"GRANT SELECT, INSERT, UPDATE, DELETE ON `{database}`.`{tab}` TO '{user}'@'{peer}'")
        params[name]=f'{user}:{password}@tcp({dbhost}:{port})/{database}?parseTime=true&loc=Asia%2FShanghai&charset=utf8mb4'
    workflow_grants(sql,database,peer)
    office_grants(sql,database,peer)
    key=sys.stdin.buffer.read().rstrip(b'\r\n').decode()
    if not key or '\r' in key or '\n' in key:raise RuntimeError('Missing order reader credential')
    params.update(OPS_READER_PASSWORD=key,OA_FORMAL_DATABASE=database,OA_FILES_DIR='/var/lib/yongbo-oa-live/files',TZ='Asia/Shanghai')
    def quote(v):return '"'+v.replace('\\','\\\\').replace('"','\\"')+'"'
    tmp=directory/'service.env.next'
    tmp.write_text(''.join(k+'='+quote(v)+'\n' for k,v in params.items()))
    tmp.chmod(0o600);tmp.replace(directory/'service.env')
    print('Scoped formal OA runtime configured.')
if __name__=='__main__':main()
