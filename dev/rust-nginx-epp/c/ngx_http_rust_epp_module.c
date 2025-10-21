/*
 * ngx_http_rust_epp_module.c
 *
 * Minimal NGINX HTTP module that invokes Rust FFI (rust_epp_get_endpoint)
 * to obtain an inference workload endpoint, sets the variable
 * "inference_workload_endpoint", and performs an internal redirect to
 * "$epp_internal_path" preserving query args.
 *
 * Directive:
 *   epp_get_endpoint on|off;
 */

#include <ngx_config.h>
#include <ngx_core.h>
#include <ngx_http.h>

/* Rust FFI functions (linked from libngx_http_rust_epp.so) */
extern int rust_epp_get_endpoint(const char* host,
                                 const char* port,
                                 const char* method,
                                 const char* headers_json,
                                 const unsigned char* body_ptr,
                                 size_t body_len,
                                 char** endpoint_out,
                                 char** error_out);
extern void rust_epp_free(char* p);

typedef struct {
    ngx_flag_t enabled;
} ngx_http_rust_epp_loc_conf_t;

/* Variable indices cached at config time */
static ngx_int_t var_idx_epp_host = NGX_ERROR;
static ngx_int_t var_idx_epp_port = NGX_ERROR;
static ngx_int_t var_idx_epp_internal_path = NGX_ERROR;
static ngx_int_t var_idx_inference_workload_endpoint = NGX_ERROR;

static char* ngx_http_rust_epp(ngx_conf_t* cf, ngx_command_t* cmd, void* conf);
static void* ngx_http_rust_epp_create_loc_conf(ngx_conf_t* cf);
static char* ngx_http_rust_epp_merge_loc_conf(ngx_conf_t* cf, void* parent, void* child);

static ngx_int_t ngx_http_rust_epp_init(ngx_conf_t* cf);
static ngx_int_t ngx_http_rust_epp_handler(ngx_http_request_t* r);

/* Helper: get variable value as ngx_str_t* (valid, not found -> NULL) */
static ngx_str_t* get_var(ngx_http_request_t* r, ngx_int_t var_idx) {
    if (var_idx == NGX_ERROR) {
        return NULL;
    }
    ngx_http_variable_value_t* vv = ngx_http_get_indexed_variable(r, var_idx);
    if (vv == NULL || vv->not_found || vv->len == 0) {
        return NULL;
    }
    /* ngx_str_t is not directly provided; we return a temporary ngx_str_t pointing to vv->data */
    static ngx_str_t s;
    s.len = vv->len;
    s.data = vv->data;
    return &s;
}

/* Helper: set variable value */
static ngx_int_t set_var(ngx_http_request_t* r, ngx_int_t var_idx, ngx_str_t* val) {
    if (var_idx == NGX_ERROR) {
        return NGX_ERROR;
    }
    ngx_http_variable_value_t* vv = ngx_http_get_indexed_variable(r, var_idx);
    if (vv == NULL) {
        return NGX_ERROR;
    }
    vv->valid = 1;
    vv->no_cacheable = 0;
    vv->not_found = 0;
    vv->len = val ? val->len : 0;
    vv->data = val ? val->data : (u_char *)"";
    return NGX_OK;
}

/* Build a very small JSON of headers with just a few entries.
 * For POC, we include "content-type" and "test-epp-endpoint-selection" if present.
 */
static ngx_str_t build_headers_json(ngx_http_request_t* r) {
    ngx_str_t json;
    /* Reserve buffer on request pool */
    u_char* buf = ngx_pnalloc(r->pool, 512);
    if (buf == NULL) {
        json.len = 2;
        json.data = (u_char*)"{}";
        return json;
    }

    ngx_uint_t written = 0;
    /* Start JSON */
    written += ngx_sprintf(buf + written, "{") - (buf + written);

    /* content-type */
    if (r->headers_in.content_type) {
        ngx_str_t ct = r->headers_in.content_type->value;
        written += ngx_sprintf(buf + written, "\"content-type\":\"%V\"", &ct) - (buf + written);
    }

    /* scan all headers for test-epp-endpoint-selection */
    ngx_list_part_t* part = &r->headers_in.headers.part;
    ngx_table_elt_t* h = part->elts;
    for (ngx_uint_t i = 0;; i++) {
        if (i >= part->nelts) {
            if (part->next == NULL) {
                break;
            }
            part = part->next;
            h = part->elts;
            i = 0;
        }
        if (h[i].key.len == sizeof("test-epp-endpoint-selection") - 1) {
            /* case-insensitive compare */
            if (ngx_strncasecmp(h[i].key.data, (u_char*)"test-epp-endpoint-selection",
                                sizeof("test-epp-endpoint-selection") - 1) == 0) {
                ngx_str_t val = h[i].value;
                if (written > 1) {
                    written += ngx_sprintf(buf + written, ",") - (buf + written);
                }
                written += ngx_sprintf(buf + written, "\"test-epp-endpoint-selection\":\"%V\"", &val) - (buf + written);
                break;
            }
        }
    }

    written += ngx_sprintf(buf + written, "}") - (buf + written);

    json.len = written;
    json.data = buf;
    return json;
}

/* Directive definition */
static ngx_command_t ngx_http_rust_epp_commands[] = {
    {
        ngx_string("epp_get_endpoint"),
        NGX_HTTP_LOC_CONF | NGX_CONF_FLAG,
        ngx_http_rust_epp,
        NGX_HTTP_LOC_CONF_OFFSET,
        offsetof(ngx_http_rust_epp_loc_conf_t, enabled),
        NULL
    },
    ngx_null_command
};

/* Module context */
static ngx_http_module_t ngx_http_rust_epp_module_ctx = {
    NULL,                          /* preconfiguration */
    ngx_http_rust_epp_init,        /* postconfiguration */

    NULL,                          /* create main configuration */
    NULL,                          /* init main configuration */

    NULL,                          /* create server configuration */
    NULL,                          /* merge server configuration */

    ngx_http_rust_epp_create_loc_conf, /* create location configuration */
    ngx_http_rust_epp_merge_loc_conf   /* merge location configuration */
};

/* Module definition */
ngx_module_t ngx_http_rust_epp_module = {
    NGX_MODULE_V1,
    &ngx_http_rust_epp_module_ctx, /* module context */
    ngx_http_rust_epp_commands,    /* module directives */
    NGX_HTTP_MODULE,               /* module type */
    NULL,                          /* init master */
    NULL,                          /* init module */
    NULL,                          /* init process */
    NULL,                          /* init thread */
    NULL,                          /* exit thread */
    NULL,                          /* exit process */
    NULL,                          /* exit master */
    NGX_MODULE_V1_PADDING
};

/* Create location conf */
static void* ngx_http_rust_epp_create_loc_conf(ngx_conf_t* cf) {
    ngx_http_rust_epp_loc_conf_t* conf;

    conf = ngx_pcalloc(cf->pool, sizeof(ngx_http_rust_epp_loc_conf_t));
    if (conf == NULL) {
        return NULL;
    }

    conf->enabled = NGX_CONF_UNSET;

    return conf;
}

/* Merge location conf */
static char* ngx_http_rust_epp_merge_loc_conf(ngx_conf_t* cf, void* parent, void* child) {
    ngx_http_rust_epp_loc_conf_t* prev = parent;
    ngx_http_rust_epp_loc_conf_t* conf = child;

    ngx_conf_merge_value(conf->enabled, prev->enabled, 0);

    return NGX_CONF_OK;
}

/* Set directive and location handler */
static char* ngx_http_rust_epp(ngx_conf_t* cf, ngx_command_t* cmd, void* conf) {
    ngx_http_core_loc_conf_t* clcf = ngx_http_conf_get_module_loc_conf(cf, ngx_http_core_module);
    ngx_http_rust_epp_loc_conf_t* lcf = conf;

    /* Parse optional on|off argument; default to on */
    if (cf->args && cf->args->nelts >= 2) {
        ngx_str_t* value = cf->args->elts;
        if (value[1].len == sizeof("off") - 1 &&
            ngx_strncasecmp(value[1].data, (u_char*)"off", sizeof("off") - 1) == 0) {
            lcf->enabled = 0;
        } else {
            lcf->enabled = 1;
        }
    } else {
        lcf->enabled = 1;
    }

    clcf->handler = ngx_http_rust_epp_handler;
    return NGX_CONF_OK;
}

/* Postconfiguration: cache variable indices */
static ngx_int_t ngx_http_rust_epp_init(ngx_conf_t* cf) {
    ngx_str_t epp_host = ngx_string("epp_host");
    ngx_str_t epp_port = ngx_string("epp_port");
    ngx_str_t epp_internal_path = ngx_string("epp_internal_path");
    ngx_str_t inference_workload_endpoint = ngx_string("inference_workload_endpoint");

    /* Ensure $inference_workload_endpoint exists and is changeable at runtime */
    ngx_http_variable_t* v = ngx_http_add_variable(cf, &inference_workload_endpoint,
                                                   NGX_HTTP_VAR_CHANGEABLE | NGX_HTTP_VAR_NOCACHEABLE);
    if (v == NULL) {
        return NGX_ERROR;
    }

    var_idx_epp_host = ngx_http_get_variable_index(cf, &epp_host);
    var_idx_epp_port = ngx_http_get_variable_index(cf, &epp_port);
    var_idx_epp_internal_path = ngx_http_get_variable_index(cf, &epp_internal_path);
    var_idx_inference_workload_endpoint = ngx_http_get_variable_index(cf, &inference_workload_endpoint);

    return NGX_OK;
}

/* Handler */
static ngx_int_t ngx_http_rust_epp_handler(ngx_http_request_t* r) {
    /* Only proceed if enabled */
    ngx_http_rust_epp_loc_conf_t* lcf = ngx_http_get_module_loc_conf(r, ngx_http_rust_epp_module);
    if (lcf == NULL || !lcf->enabled) {
        return NGX_DECLINED;
    }

    /* Fetch required variables */
    ngx_str_t* host = get_var(r, var_idx_epp_host);
    ngx_str_t* port = get_var(r, var_idx_epp_port);
    ngx_str_t* internal_path = get_var(r, var_idx_epp_internal_path);

    if (host == NULL || port == NULL || internal_path == NULL) {
        ngx_log_error(NGX_LOG_ERR, r->connection->log, 0,
                      "rust_epp: missing required variables epp_host/epp_port/epp_internal_path");
        return NGX_HTTP_BAD_REQUEST;
    }

    /* Build minimal headers JSON */
    ngx_str_t headers_json = build_headers_json(r);

    /* Method */
    ngx_str_t method = r->method_name;

    /* No body for POC (can be extended to read client body) */
    char* endpoint_out = NULL;
    char* error_out = NULL;

    /* Convert ngx_str_t to C strings (null-terminated in temporary buffers) */
    u_char* hostz = ngx_pnalloc(r->pool, host->len + 1);
    u_char* portz = ngx_pnalloc(r->pool, port->len + 1);
    u_char* methodz = ngx_pnalloc(r->pool, method.len + 1);
    u_char* headersz = ngx_pnalloc(r->pool, headers_json.len + 1);
    if (hostz == NULL || portz == NULL || methodz == NULL || headersz == NULL) {
        return NGX_HTTP_INTERNAL_SERVER_ERROR;
    }
    ngx_memcpy(hostz, host->data, host->len); hostz[host->len] = '\0';
    ngx_memcpy(portz, port->data, port->len); portz[port->len] = '\0';
    ngx_memcpy(methodz, method.data, method.len); methodz[method.len] = '\0';
    ngx_memcpy(headersz, headers_json.data, headers_json.len); headersz[headers_json.len] = '\0';

    int rc = rust_epp_get_endpoint((const char*)hostz,
                                   (const char*)portz,
                                   (const char*)methodz,
                                   (const char*)headersz,
                                   NULL, 0,
                                   &endpoint_out,
                                   &error_out);

    if (rc != 0) {
        if (error_out) {
            ngx_log_error(NGX_LOG_ERR, r->connection->log, 0,
                          "rust_epp_get_endpoint error: %s", error_out);
            rust_epp_free(error_out);
        } else {
            ngx_log_error(NGX_LOG_ERR, r->connection->log, 0,
                          "rust_epp_get_endpoint error: rc=%d", rc);
        }
        /* Continue with internal redirect regardless (fail-open/close handled by NGF config) */
    } else if (endpoint_out) {
        /* Set $inference_workload_endpoint */
        size_t elen = ngx_strlen(endpoint_out);
        u_char* eb = ngx_pnalloc(r->pool, elen);
        if (eb != NULL) {
            ngx_memcpy(eb, endpoint_out, elen);
            ngx_str_t ev;
            ev.len = elen;
            ev.data = eb;
            (void)set_var(r, var_idx_inference_workload_endpoint, &ev);
        }
        rust_epp_free(endpoint_out);
    }

    /* Perform internal redirect to $epp_internal_path with args preserved */
    ngx_str_t uri = *internal_path;
    ngx_str_t args = r->args;

    ngx_http_internal_redirect(r, &uri, &args);
    return NGX_DONE;
}
