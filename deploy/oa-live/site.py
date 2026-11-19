#!/usr/bin/env python3
"""Derive only the OA virtual host from the active main-site nginx contract."""
import re
import sys
from pathlib import Path

# nginx inherits add_header only when the child defines no headers of its own.
def secure_headers(vhost):
    directives = {
        'Strict-Transport-Security': 'max-age=31536000',
        'X-Frame-Options': 'SAMEORIGIN',
        'X-Content-Type-Options': 'nosniff',
        'Referrer-Policy': 'strict-origin-when-cross-origin',
        'Content-Security-Policy': "base-uri 'self'; frame-ancestors 'self'; object-src 'self' blob:; script-src 'self' 'wasm-unsafe-eval'; worker-src 'self' blob:; form-action 'self'",
    }
    names = '|'.join(map(re.escape, directives))
    vhost = re.sub(r'^\s*add_header\s+(?:'+names+r')\s+[^;\n]+;[ \t]*$', '', vhost, flags=re.M|re.I)
    def headers(indent):
        return '\n'.join(f'{indent}add_header {name} "{value}" always;' for name,value in directives.items())
    # The copied vhost has flat location blocks. Preserve native cache headers.
    def location(match):
        block = match.group()
        return block[:-1] + headers('        ') + '\n    }' if re.search(r'^\s*add_header\s+', block, re.M) else block
    vhost = PROXY_LOCATION.sub(location, vhost)
    return vhost.replace('server {', 'server {\n'+headers('    '), 1)

PROXY_LOCATION = re.compile(r'^    location[^\n]*\{[ \t]*\n.*?^    \}', re.M | re.S)
CORE_PROXY = re.compile(r'(?m)^\s*proxy_pass http://yongbo_main_api(?:/[a-z]+)?;\s*$')
SESSION_PROXY = 'http://127.0.0.1:18095'

# These maps are scoped to the OA vhost by their use in its generated locations.
# Only the documented 32-byte core session format enters Authorization.
SESSION_MAPS = """
map $http_cookie $yb_oa_session_authorization {
    default "";
    "~(?:^|;[ \\t]*)__Host-yb-oa-session=(?<yb_oa_session_value>[0-9a-f]{64})(?:;|$)" "Bearer $yb_oa_session_value";
}
map $yb_oa_session_authorization $yb_oa_migration_cookie {
    default "";
    "~^Bearer (?<yb_oa_migration_value>[0-9a-f]{64})$" "__Host-yb-oa-session=$yb_oa_migration_value";
}
map "$http_sec_fetch_site:$http_origin:$http_x_frontend_version" $yb_oa_browser_request {
    "::" 0;
    default 1;
}
map "$yb_oa_browser_request:$yb_oa_session_authorization:$http_authorization" $yb_oa_api_bearer {
    default "";
    "~*^0::(?<yb_oa_api_value>Bearer [0-9a-f]{64})$" $yb_oa_api_value;
}
map $yb_oa_api_bearer $yb_oa_upstream_authorization {
    "" $yb_oa_session_authorization;
    default $yb_oa_api_bearer;
}
map "$yb_oa_api_bearer:$http_authorization" $yb_oa_block_explicit_auth {
    default 0;
    ~^:.+ 1;
}
map "$yb_oa_api_bearer:$request_method:$http_origin" $yb_oa_foreign_mutation {
    default 1;
    ~*^Bearer 0;
    ~^:(?:GET|HEAD|OPTIONS): 0;
    ~^:[A-Z]+:https://oa\\.yongbo\\.cloud$ 0;
}
map "$yb_oa_api_bearer:$http_origin" $yb_oa_foreign_socket {
    default 1;
    ~*^Bearer 0;
    ~^:https://oa\\.yongbo\\.cloud$ 0;
}
"""


def add_proxy_directives(block, directives):
    opening, body = block.split('\n', 1)
    return opening + '\n' + ''.join(f'        {directive}\n' for directive in directives) + body


def strip_external_cookies(vhost):
    def location(match):
        block = match.group()
        if 'proxy_pass' not in block or CORE_PROXY.search(block) or 'Cookie $yb_oa_migration_cookie;' in block:
            return block
        block = re.sub(r'(?mi)^\s*proxy_(?:set_header Cookie|hide_header Set-Cookie)\s*[^;\n]*;[ \t]*$', '', block)
        directives = ['proxy_set_header Cookie "";']
        opening = block.splitlines()[0]
        cookie_issuer = f'proxy_pass {SESSION_PROXY};' in block and (
            '/v1/auth/(?:login|register)' in opening or '/v1/auth/(?:asset-cookie|logout)' in opening
        )
        if not cookie_issuer:
            directives.append('proxy_hide_header Set-Cookie;')
        if '/_protected/' in opening:
            block = re.sub(r'(?mi)^\s*proxy_set_header Authorization\s*[^;\n]*;[ \t]*$', '', block)
            directives.append('proxy_set_header Authorization "";')
        return add_proxy_directives(block, directives)
    return PROXY_LOCATION.sub(location, vhost)


def protect_rollback(vhost):
    """Preserve P1 path cookies while excluding P2's host cookie from every upstream."""
    vhost = strip_external_cookies(vhost)
    if '$yb_oa_rollback_cookie' in vhost:
        return vhost
    def location(match):
        block = match.group()
        if CORE_PROXY.search(block) and 'proxy_set_header Cookie' not in block:
            return add_proxy_directives(block, ['proxy_set_header Cookie $yb_oa_rollback_cookie;'])
        return block
    result = PROXY_LOCATION.sub(location, vhost)
    if result == vhost:
        return vhost
    return '''
map "$cookie_wf_asset_token:$cookie_wf_asset_stream_token:$cookie_wf_ws_token" $yb_oa_rollback_cookie {
    "::" "";
    default "wf_asset_token=$cookie_wf_asset_token; wf_asset_stream_token=$cookie_wf_asset_stream_token; wf_ws_token=$cookie_wf_ws_token";
}
''' + result


def browser_sessions(vhost):
    def location(match):
        block = match.group()
        if not CORE_PROXY.search(block):
            return block
        opening = block.splitlines()[0]
        broker = '/v1/auth/login' in opening or 'asset-cookie|logout' in opening
        websocket = '/ws/' in opening
        api = '/v1/' in opening
        block = re.sub(r'(?mi)^\s*proxy_(?:set_header Cookie|hide_header Set-Cookie)\s*[^;\n]*;[ \t]*$', '', block)
        directives = ['proxy_set_header Cookie "";']
        if not broker:
            directives.append('proxy_hide_header Set-Cookie;')
        if api or websocket:
            block = re.sub(r'(?mi)^\s*proxy_set_header Authorization\s*[^;\n]*;[ \t]*$', '', block)
            directives.append('proxy_set_header Authorization $yb_oa_upstream_authorization;')
        if api or websocket:
            directives.append('if ($yb_oa_block_explicit_auth) { return 409; }')
        if api:
            directives.append('if ($yb_oa_foreign_mutation) { return 403; }')
        if websocket:
            directives += ['if ($yb_oa_foreign_socket) { return 403; }',
                           'if ($http_sec_websocket_protocol ~ "wf-token\\.") { return 401; }']
        if broker:
            browser_directives = [
                directive.replace('$yb_oa_upstream_authorization', '$yb_oa_session_authorization')
                for directive in directives
                if '$yb_oa_block_explicit_auth' not in directive
            ]
            browser_directives += ['if ($http_authorization != "") { return 409; }',
                                   'if ($http_origin != "https://oa.yongbo.cloud") { return 403; }']
            if '/v1/auth/login' in opening:
                block = block.replace('location = /v1/auth/login', 'location ~ ^/v1/auth/(?:login|register)$')
            else:
                me = block.replace('location ~ ^/v1/auth/(?:me|asset-cookie|logout)$', 'location = /v1/auth/me')
                me = add_proxy_directives(me, directives + ['proxy_hide_header Set-Cookie;'])
                session = block.replace('(?:me|asset-cookie|logout)', '(?:asset-cookie|logout)')
                session = session.replace('proxy_pass http://yongbo_main_api;', f'proxy_pass {SESSION_PROXY};')
                return me + '\n\n' + add_proxy_directives(session, browser_directives)
            block = block.replace('proxy_pass http://yongbo_main_api;', f'proxy_pass {SESSION_PROXY};')
            return add_proxy_directives(block, browser_directives)
        return add_proxy_directives(block, directives)
    return PROXY_LOCATION.sub(location, vhost)



TRACE_CONFIG = """
log_format yb_oa_trace 'trace_id=$request_id status=$status upstream_status=$upstream_status';
map $http_x_oa_error_format $yb_oa_standard_error {
    default 0;
    v2 1;
}
"""

TRACE_ERRORS = {
    400: ('BAD_REQUEST', '请求参数有误，请检查后重试'),
    401: ('UNAUTHENTICATED', '登录状态已失效，请重新登录'),
    403: ('FORBIDDEN', '暂无权限执行该操作'),
    404: ('NOT_FOUND', '请求的资源不存在'),
    405: ('METHOD_NOT_ALLOWED', '当前入口不支持该操作'),
    408: ('DEADLINE_EXCEEDED', '请求超时，请稍后重试'),
    409: ('OA_CLIENT_REFRESH_REQUIRED', 'OA 登录方式已更新，请刷新页面后继续'),
    413: ('PAYLOAD_TOO_LARGE', '上传内容超过大小限制'),
    429: ('RESOURCE_EXHAUSTED', '请求过于频繁，请稍后再试'),
    500: ('INTERNAL_ERROR', '服务暂时不可用，请稍后重试'),
    502: ('UNAVAILABLE', '服务暂时不可用，请稍后重试'),
    503: ('UNAVAILABLE', '服务暂时不可用或繁忙，请稍后重试'),
    504: ('DEADLINE_EXCEEDED', '服务响应超时，请刷新页面核对操作结果后再试'),
}


def trace_api(vhost):
    trace_headers = [
        'add_header X-Trace-ID $request_id always;',
        'access_log /var/log/nginx/yongbo-oa-api.access.log yb_oa_trace;',
        # The default nginx upstream diagnostic can print the complete URI.
        'error_log /var/log/nginx/yongbo-oa-api.error.log crit;',
    ]

    def location(match):
        block = match.group()
        opening = block.splitlines()[0]
        if not any(prefix in opening for prefix in ('/v1/', '/oa-api/')):
            return block
        block = re.sub(r'(?mi)^\s*(?:proxy_set_header|add_header)\s+X-Trace-ID\s+[^;\n]+;', '', block)
        block = re.sub(r'(?m)^\s*(?:access_log|error_log|proxy_intercept_errors)\s+[^;\n]+;', '', block)
        return add_proxy_directives(block, trace_headers + [
            'proxy_set_header X-Trace-ID $request_id;',
            'proxy_hide_header X-Trace-ID;',
            'proxy_intercept_errors off;',
        ] + [f'error_page {status} = @yb_oa_api_{status};' for status in TRACE_ERRORS])

    handlers = ''
    for status, (code, message) in TRACE_ERRORS.items():
        envelope = '{"error":{"code":"' + code + '","message":"' + message + '","trace_id":"$request_id"}}'
        reply = f"return {status} '{envelope}';"
        if status != 409:
            reply = f'if ($yb_oa_standard_error) {{ {reply} }}\n        return {status};'
        handlers += f"""
    location @yb_oa_api_{status} {{
        default_type application/json;
        add_header Vary X-OA-Error-Format always;
        add_header Cache-Control no-store always;
        {reply}
    }}
"""
        marker = f'    location @yb_oa_api_{status} {{\n'
        handlers = handlers.replace(marker, marker + ''.join(f'        {line}\n' for line in trace_headers), 1)
    return PROXY_LOCATION.sub(location, vhost)[:-2] + handlers + '}\n'


def render(text,root,previous=None):
    servers=re.findall(r'^server\s*\{.*?^\}',text,re.M|re.S)
    choices=[s for s in servers if re.search(r'^\s*server_name\s+yongbo\.cloud;',s,re.M) and re.search(r'listen\s+443\s+ssl',s)]
    if len(choices)!=1:raise ValueError('Expected one main HTTPS server block')
    vhost=choices[0]
    # ipv6only configures the shared listening socket and may appear only once.
    vhost=re.sub(r'(?m)^(\s*listen\s+\[::\]:443\s+ssl)\s+ipv6only=on;',r'\1;',vhost)
    required = ('root /var/www/yongbo.cloud;', 'location /v1/ {', 'location ^~ /ws/ {',
                'location = /v1/auth/login {', 'location ~ ^/v1/auth/(?:me|asset-cookie|logout)$ {')
    if not all(marker in vhost for marker in required):
        raise ValueError('Main deployment layout changed')
    # A native API URL may end in .png or another static file extension.
    vhost = vhost.replace(r'location ~* \.(?:', r'location ~* ^(?!/v1/).*\.(?:')
    vhost=vhost.replace('server_name yongbo.cloud;','server_name oa.yongbo.cloud;').replace('/etc/letsencrypt/live/yongbo.cloud/','/etc/letsencrypt/live/oa.yongbo.cloud/').replace('root /var/www/yongbo.cloud;',f'root {root};')
    vhost = browser_sessions(strip_external_cookies(vhost))
    extra="""
    location = /oa-api/session/migrate {
        limit_req zone=yongbo_auth_per_ip burst=20 nodelay;
        proxy_pass http://127.0.0.1:18095;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto https;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header Cookie $yb_oa_migration_cookie;
        proxy_set_header Authorization $http_authorization;
        if ($yb_oa_foreign_mutation) { return 403; }
    }
    location = /oa-api/health {
        return 404;
    }
    location ^~ /oa-api/ {
        proxy_pass http://127.0.0.1:18095;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto https;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header Cookie "";
        proxy_set_header Authorization $yb_oa_upstream_authorization;
        proxy_hide_header Set-Cookie;
        if ($yb_oa_block_explicit_auth) { return 409; }
        if ($yb_oa_foreign_mutation) { return 403; }
        client_max_body_size 12m;
        proxy_read_timeout 65s;
    }
    location = /oa.html { return 302 /; }
    location = /oa-work.html { return 302 /tasks; }
    location = /oa-assets.html { return 410; }
    location ^~ /oa-objects/ { return 410; }
"""
    if previous:
        if not re.fullmatch(r'/srv/yongbo-oa-live/releases/[A-Za-z0-9-]+/web', previous):
            raise ValueError('Unexpected OA previous release directory')
        extra += """
    location ^~ /assets/ {
        try_files $uri @oa_previous_asset;
        expires 1y;
    }
    location @oa_previous_asset {
        root """ + previous + """;
        try_files $uri =404;
        expires 1y;
    }
"""
    return SESSION_MAPS + TRACE_CONFIG + """server {
    listen 80;
    listen [::]:80;
    server_name oa.yongbo.cloud;
    location ^~ /.well-known/acme-challenge/ { root /var/www/certbot; try_files $uri =404; }
    location / { return 301 https://oa.yongbo.cloud$request_uri; }
}
"""+secure_headers(trace_api(vhost[:-1]+extra+"}\n"))
if __name__=='__main__':
    if sys.argv[1] == '--protect-rollback':
        print(protect_rollback(Path(sys.argv[2]).read_text()), end='')
    else:
        print(render(Path(sys.argv[1]).read_text(),'/srv/yongbo-oa-live/current/web',sys.argv[2] if len(sys.argv)>2 else None))
