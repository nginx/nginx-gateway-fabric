//! Endpoint address parsing.
//!
//! An endpoint is `host:port` where host may be an IPv4 address, an
//! IPv6 address in brackets, or a hostname. The EPP may return a
//! comma-separated ordered list. Parsing preserves the order; routing
//! currently uses only the first entry.

use std::net::{Ipv4Addr, Ipv6Addr};

use crate::error::ModuleError;

/// A parsed `host:port` endpoint.
#[derive(Debug, Clone, PartialEq, Eq)]
pub(crate) struct Endpoint {
    /// The full `host:port` string (or `[ipv6]:port`).
    pub address: String,
}

/// Split `host:port` or `[ipv6]:port` into host (without brackets) and a
/// non-zero port.
///
/// Unbracketed hosts must not contain `:`, so a bare IPv6 address is
/// rejected rather than misparsed.
pub(crate) fn split_host_port(s: &str) -> Result<(&str, u16), String> {
    let (host, port_str) = if let Some(rest) = s.strip_prefix('[') {
        let (host, tail) = rest
            .split_once(']')
            .ok_or_else(|| format!("missing closing bracket in '{s}'"))?;
        host.parse::<Ipv6Addr>()
            .map_err(|_| format!("invalid IPv6 host in '{s}'"))?;
        let port_str = tail
            .strip_prefix(':')
            .ok_or_else(|| format!("missing port in '{s}'"))?;
        (host, port_str)
    } else {
        let (host, port_str) = s
            .split_once(':')
            .ok_or_else(|| format!("missing port in '{s}'"))?;
        if host.contains(':') || port_str.contains(':') {
            return Err(format!("IPv6 address must be bracketed in '{s}'"));
        }
        if !is_valid_host(host) {
            return Err(format!("invalid IP or DNS host in '{s}'"));
        }
        (host, port_str)
    };

    if host.is_empty() {
        return Err(format!("empty host in '{s}'"));
    }
    if port_str.is_empty() || !port_str.bytes().all(|b| b.is_ascii_digit()) {
        return Err(format!("invalid port in '{s}'"));
    }
    let port: u16 = port_str
        .parse()
        .map_err(|_| format!("invalid port in '{s}'"))?;
    if port == 0 {
        return Err(format!("port 0 in '{s}'"));
    }
    Ok((host, port))
}

fn is_valid_host(host: &str) -> bool {
    if host.parse::<Ipv4Addr>().is_ok() {
        return true;
    }
    let host = host.strip_suffix('.').unwrap_or(host);
    if host.is_empty() || host.len() > 253 {
        return false;
    }
    // Numeric dotted hosts must be valid IPv4, not ambiguous DNS names.
    if host.bytes().all(|b| b.is_ascii_digit() || b == b'.') {
        return false;
    }
    host.split('.').all(|label| {
        !label.is_empty()
            && label.len() <= 63
            && label.as_bytes()[0].is_ascii_alphanumeric()
            && label.as_bytes()[label.len() - 1].is_ascii_alphanumeric()
            && label
                .bytes()
                .all(|b| b.is_ascii_alphanumeric() || b == b'-')
    })
}

/// Parse a single `host:port` endpoint.
///
/// Accepts `1.2.3.4:8080`, `[::1]:8080`, or `hostname:8080`.
pub(crate) fn parse_endpoint(s: &str) -> Result<Endpoint, ModuleError> {
    let s = s.trim_matches(' ');
    if s.is_empty() {
        return Err(ModuleError::InvalidResponse("empty endpoint string".into()));
    }
    split_host_port(s).map_err(ModuleError::InvalidResponse)?;
    Ok(Endpoint {
        address: s.to_owned(),
    })
}

/// Parse a comma-separated list of endpoints. At least one is required.
pub(crate) fn parse_endpoint_list(raw: &str) -> Result<Vec<Endpoint>, ModuleError> {
    let endpoints: Result<Vec<_>, _> = raw.split(',').map(parse_endpoint).collect();
    let endpoints = endpoints?;
    if endpoints.is_empty() {
        return Err(ModuleError::InvalidResponse("empty endpoint list".into()));
    }
    Ok(endpoints)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn ipv4() {
        let ep = parse_endpoint("10.0.0.1:8080").unwrap();
        assert_eq!(ep.address, "10.0.0.1:8080");
    }

    #[test]
    fn ipv6_bracket() {
        let ep = parse_endpoint("[::1]:9002").unwrap();
        assert_eq!(ep.address, "[::1]:9002");
    }

    #[test]
    fn ipv6_full() {
        let ep = parse_endpoint("[2001:db8::1]:443").unwrap();
        assert_eq!(ep.address, "[2001:db8::1]:443");
    }

    #[test]
    fn hostname() {
        let ep = parse_endpoint("model-server.ns:8080").unwrap();
        assert_eq!(ep.address, "model-server.ns:8080");
    }

    #[test]
    fn whitespace_trimmed() {
        let ep = parse_endpoint("  10.0.0.1:80  ").unwrap();
        assert_eq!(ep.address, "10.0.0.1:80");
    }

    #[test]
    fn missing_port() {
        assert!(parse_endpoint("10.0.0.1").is_err());
    }

    #[test]
    fn empty_host() {
        assert!(parse_endpoint(":8080").is_err());
    }

    #[test]
    fn port_zero() {
        assert!(parse_endpoint("10.0.0.1:0").is_err());
    }

    #[test]
    fn port_non_numeric() {
        assert!(parse_endpoint("10.0.0.1:abc").is_err());
    }

    #[test]
    fn empty_string() {
        assert!(parse_endpoint("").is_err());
    }

    #[test]
    fn ipv6_missing_bracket() {
        assert!(parse_endpoint("[::1:8080").is_err());
    }

    #[test]
    fn ipv6_no_port() {
        assert!(parse_endpoint("[::1]").is_err());
    }

    #[test]
    fn ipv6_empty_addr() {
        assert!(parse_endpoint("[]:8080").is_err());
    }

    #[test]
    fn comma_list() {
        let eps = parse_endpoint_list("10.0.0.1:80, 10.0.0.2:80, [::1]:80").unwrap();
        assert_eq!(eps.len(), 3);
        assert_eq!(eps[0].address, "10.0.0.1:80");
        assert_eq!(eps[1].address, "10.0.0.2:80");
        assert_eq!(eps[2].address, "[::1]:80");
    }

    #[test]
    fn single_element_list() {
        let eps = parse_endpoint_list("host:443").unwrap();
        assert_eq!(eps.len(), 1);
    }

    #[test]
    fn split_ipv4_and_hostname() {
        assert_eq!(
            split_host_port("epp.ns.svc.cluster.local:9002").unwrap(),
            ("epp.ns.svc.cluster.local", 9002)
        );
    }

    #[test]
    fn split_ipv6() {
        assert_eq!(split_host_port("[::1]:8080").unwrap(), ("::1", 8080));
    }

    #[test]
    fn split_rejects_unbracketed_ipv6() {
        assert!(split_host_port("::1:8080").is_err());
        assert!(split_host_port("fe80::1").is_err());
    }

    #[test]
    fn split_rejects_bad_ports() {
        assert!(split_host_port("hostname-only").is_err());
        assert!(split_host_port("host:notanumber").is_err());
        assert!(split_host_port("host:99999").is_err());
        assert!(split_host_port("host:").is_err());
    }

    #[test]
    fn list_with_bad_entry() {
        assert!(parse_endpoint_list("10.0.0.1:80, bad, 10.0.0.2:80").is_err());
    }

    #[test]
    fn strict_endpoint_syntax() {
        for invalid in [
            "[hostname]:80",
            "[127.0.0.1]:80",
            "[gggg::1]:80",
            "[::1]]:80",
            "[fe80::1%eth0]:80",
            "user@host:80",
            "http://host:80",
            "host/path:80",
            "host?query:80",
            "host#fragment:80",
            "host\\path:80",
            "host%2fpath:80",
            "host;param:80",
            "host:80/path",
            "host:+80",
            "host:-80",
            "host:８０",
            "host: 80",
            "host:0",
            "host:65536",
            "host\t:80",
            "host:80\r\n",
            "\nhost:80",
            "host:80\0",
            "host:80\u{7f}",
            "ho st:80",
            "höst:80",
            "host_name:80",
            ".host:80",
            "host..name:80",
            "host..:80",
            "-host:80",
            "host-:80",
            "256.0.0.1:80",
            "127.01.0.1:80",
            "[::1]:+80",
            "[::1]:80?query",
            "host,:80",
        ] {
            assert!(parse_endpoint(invalid).is_err(), "accepted {invalid:?}");
            assert!(
                split_host_port(invalid).is_err(),
                "split accepted {invalid:?}"
            );
        }
        for valid in [
            "localhost:1",
            "Model-Server.ns.svc.cluster.local.:65535",
            "host:00080",
            "127.0.0.1:80",
            "[::ffff:192.0.2.1]:80",
        ] {
            assert!(parse_endpoint(valid).is_ok(), "rejected {valid:?}");
        }
    }

    #[test]
    fn dns_length_limits() {
        let label = "a".repeat(63);
        let host = format!("{label}.{label}.{label}.{}", "b".repeat(61));
        assert_eq!(host.len(), 253);
        assert!(parse_endpoint(&format!("{host}.:80")).is_ok());
        assert!(parse_endpoint(&format!("{host}b:80")).is_err());
        assert!(parse_endpoint(&format!("{}:80", "a".repeat(64))).is_err());
    }

    #[test]
    fn endpoint_lists_reject_empty_or_unsafe_entries() {
        for invalid in [
            "",
            ",",
            "host:80,",
            ",host:80",
            "host:80,,other:80",
            "host:80,\nother:80",
        ] {
            assert!(
                parse_endpoint_list(invalid).is_err(),
                "accepted {invalid:?}"
            );
        }
    }
}
