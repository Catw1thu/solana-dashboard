use anyhow::{Result, bail};
use tokio::{
    io::{AsyncReadExt, AsyncWriteExt},
    net::TcpStream,
};

use super::model::ServiceEventEnvelope;

pub struct ServiceEventEmitter {
    target: Option<HttpTarget>,
}

impl ServiceEventEmitter {
    pub fn new(raw_url: Option<&str>) -> Result<Self> {
        let target = match raw_url {
            Some(url) => Some(HttpTarget::parse(url)?),
            None => None,
        };

        Ok(Self { target })
    }

    pub async fn emit(&self, event: &ServiceEventEnvelope) -> Result<()> {
        let Some(target) = &self.target else {
            return Ok(());
        };

        let body = serde_json::to_vec(event)?;
        let mut stream = TcpStream::connect((target.host.as_str(), target.port)).await?;

        let request = format!(
            "POST {} HTTP/1.1\r\nHost: {}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
            target.path,
            target.host_header(),
            body.len()
        );

        stream.write_all(request.as_bytes()).await?;
        stream.write_all(&body).await?;

        let mut response = Vec::new();
        stream.read_to_end(&mut response).await?;

        let status = parse_status_code(&response)?;
        if !(200..300).contains(&status) {
            let response_text = String::from_utf8_lossy(&response);
            bail!(
                "service event ingest returned status {} for {}:{}{}: {}",
                status,
                target.host,
                target.port,
                target.path,
                response_text.trim()
            );
        }

        Ok(())
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
struct HttpTarget {
    host: String,
    port: u16,
    path: String,
}

impl HttpTarget {
    fn parse(url: &str) -> Result<Self> {
        let stripped = url
            .strip_prefix("http://")
            .ok_or_else(|| anyhow::anyhow!("only http:// URLs are supported"))?;

        let (host_port, path) = match stripped.split_once('/') {
            Some((host_port, rest)) => (host_port, format!("/{}", rest)),
            None => (stripped, "/".to_string()),
        };

        if host_port.is_empty() {
            bail!("ingest URL host is empty");
        }

        let (host, port) = match host_port.rsplit_once(':') {
            Some((host, port)) if !host.is_empty() && !port.is_empty() => {
                let port = port.parse::<u16>()?;
                (host.to_string(), port)
            }
            _ => (host_port.to_string(), 80),
        };

        Ok(Self { host, port, path })
    }

    fn host_header(&self) -> String {
        if self.port == 80 {
            self.host.clone()
        } else {
            format!("{}:{}", self.host, self.port)
        }
    }
}

fn parse_status_code(response: &[u8]) -> Result<u16> {
    let response = String::from_utf8_lossy(response);
    let status_line = response
        .lines()
        .next()
        .ok_or_else(|| anyhow::anyhow!("missing HTTP status line"))?;

    let mut parts = status_line.split_whitespace();
    let _http_version = parts
        .next()
        .ok_or_else(|| anyhow::anyhow!("missing HTTP version"))?;
    let status = parts
        .next()
        .ok_or_else(|| anyhow::anyhow!("missing HTTP status code"))?;

    Ok(status.parse::<u16>()?)
}

#[cfg(test)]
mod tests {
    use super::{HttpTarget, parse_status_code};

    #[test]
    fn parses_http_target_with_explicit_port() {
        let target = HttpTarget::parse("http://127.0.0.1:8080/internal/events").unwrap();
        assert_eq!(target.host, "127.0.0.1");
        assert_eq!(target.port, 8080);
        assert_eq!(target.path, "/internal/events");
        assert_eq!(target.host_header(), "127.0.0.1:8080");
    }

    #[test]
    fn parses_http_target_with_default_path() {
        let target = HttpTarget::parse("http://localhost").unwrap();
        assert_eq!(target.host, "localhost");
        assert_eq!(target.port, 80);
        assert_eq!(target.path, "/");
        assert_eq!(target.host_header(), "localhost");
    }

    #[test]
    fn parses_http_status_code() {
        let response = b"HTTP/1.1 202 Accepted\r\nContent-Length: 0\r\n\r\n";
        let status = parse_status_code(response).unwrap();
        assert_eq!(status, 202);
    }
}
