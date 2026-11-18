#!/usr/bin/env python3
"""Derive only the OA virtual host from the active main-site nginx contract."""
import re
import sys
from pathlib import Path

def render(text,root):
    servers=re.findall(r'^server\s*\{.*?^\}',text,re.M|re.S)
    choices=[s for s in servers if re.search(r'^\s*server_name\s+yongbo\.cloud;',s,re.M) and re.search(r'listen\s+443\s+ssl',s)]
    if len(choices)!=1:raise ValueError('Expected one main HTTPS server block')
    vhost=choices[0]
    # ipv6only configures the shared listening socket and may appear only once.
    vhost=re.sub(r'(?m)^(\s*listen\s+\[::\]:443\s+ssl)\s+ipv6only=on;',r'\1;',vhost)
    if '/var/www/yongbo.cloud' not in vhost or 'location /v1/' not in vhost:raise ValueError('Main deployment layout changed')
    vhost=vhost.replace('server_name yongbo.cloud;','server_name oa.yongbo.cloud;').replace('/etc/letsencrypt/live/yongbo.cloud/','/etc/letsencrypt/live/oa.yongbo.cloud/').replace('root /var/www/yongbo.cloud;',f'root {root};')
    extra="""
    proxy_cookie_domain yongbo.cloud oa.yongbo.cloud;
    location ^~ /oa-api/ {
        proxy_pass http://127.0.0.1:18095;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto https;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header Cookie "";
        client_max_body_size 12m;
        proxy_read_timeout 65s;
    }
    location = /oa.html { return 302 /; }
    location = /oa-work.html { return 302 /tasks; }
    location = /oa-assets.html { return 410; }
    location ^~ /oa-objects/ { return 410; }
"""
    return """server {
    listen 80;
    listen [::]:80;
    server_name oa.yongbo.cloud;
    location ^~ /.well-known/acme-challenge/ { root /var/www/certbot; try_files $uri =404; }
    location / { return 301 https://oa.yongbo.cloud$request_uri; }
}
"""+vhost[:-1]+extra+"}\n"
if __name__=='__main__':
    print(render(Path(sys.argv[1]).read_text(),'/srv/yongbo-oa-live/current/web'))
