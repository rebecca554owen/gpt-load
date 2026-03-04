# Security Policy

## Supported Versions

The GPT-Load team is committed to providing security updates for the following versions:

| Version | Supported          | Notes                     |
| ------- | ------------------ | ------------------------- |
| 1.5.x   | :white_check_mark: | Latest stable series      |
| 1.4.x   | :white_check_mark: | Security fixes only       |
| < 1.4   | :x:                | End of life               |

**Version Support Policy:**
- Only the latest minor version in each series receives security updates
- Users are strongly encouraged to upgrade to the latest stable release (1.5.x)
- Security patches for older versions (1.4.x) will be provided for 3 months after a new major release
- Critical vulnerabilities may warrant exceptions to this policy

## Reporting a Vulnerability

If you discover a security vulnerability in GPT-Load, we appreciate your responsible disclosure.

### How to Report

**Preferred Method:** Send a detailed report to [GitHub Security Advisories](https://github.com/gpt-load/gpt-load/security/advisories)

**Alternative:** Contact the maintainers privately via email if the issue is extremely sensitive

### What to Include

Please provide as much information as possible:
- Description of the vulnerability
- Steps to reproduce the issue
- Potential impact assessment
- Suggested mitigation (if any)
- Proof of concept (optional but helpful)

### Response Timeline

- **Initial Response:** Within 48 hours
- **Detailed Assessment:** Within 7 days
- **Resolution:** Dependent on severity and complexity

### Vulnerability Handling Process

1. **Acknowledgment:** You will receive confirmation that your report has been received
2. **Validation:** The team will validate and assess the severity of the vulnerability
3. **Coordination:** We will work with you to develop a fix (if accepted)
4. **Disclosure:** A security advisory will be published when a fix is available
5. **Credit:** Vulnerability reporters will be credited in the advisory (upon request)

### Accepted vs Declined

- **Accepted:** We will develop a patch and coordinate disclosure timing
- **Declined:** You will receive an explanation (e.g., intended behavior, out of scope, duplicate)

## Security Best Practices

### Deployment Security

**1. Change Default Authentication**

The default `AUTH_KEY` is publicly known and must be changed immediately upon deployment:

```bash
export AUTH_KEY="your-strong-random-key-here"
```

Generate a secure key using:
```bash
openssl rand -hex 32
```

**2. Enable API Key Encryption**

Enable encryption for stored API keys to protect sensitive credentials:

```bash
export ENCRYPTION_KEY="your-32-byte-hex-key"
```

Generate an encryption key:
```bash
openssl rand -hex 32
```

**Note:** When enabling encryption for the first time, use the migration command:
```bash
make migrate-keys ARGS="--to $ENCRYPTION_KEY"
```

**3. Database Security**

- Use strong database credentials
- Restrict database access to localhost or trusted networks
- Enable SSL/TLS for remote database connections
- Regularly backup your database

**4. Network Security**

- Deploy behind a reverse proxy (nginx, Caddy, etc.)
- Enable HTTPS with valid certificates
- Restrict access to administrative endpoints
- Configure firewall rules to limit exposure

**5. Runtime Security**

- Run as a non-root user whenever possible
- Keep dependencies updated
- Monitor logs for suspicious activity
- Implement rate limiting on public endpoints

**6. Master/Slave Configuration**

When running multiple instances:
- Only one master node should perform database migrations
- Configure `IS_SLAVE=true` for worker nodes
- Ensure all nodes share the same database and Redis

### Environment Variables

| Variable | Description | Default | Secure? |
|----------|-------------|---------|---------|
| `AUTH_KEY` | Proxy authentication key | `sk-xxx` | **NO** - Change immediately |
| `ENCRYPTION_KEY` | API key encryption | Empty | Recommended |
| `DB_DSN` | Database connection | - | Keep secret |
| `REDIS_URI` | Redis connection | - | Keep secret |
| `JWT_SECRET` | JWT signing key | - | Keep secret |

### Operational Security

1. **Credential Rotation:** Regularly rotate API keys and authentication tokens
2. **Access Control:** Implement proper access controls for the web interface
3. **Audit Logging:** Enable and monitor request logs for security events
4. **Update Regularly:** Keep GPT-Load updated to the latest stable release

## Security Features

GPT-Load includes several built-in security features:

- **Constant-time comparison** for authentication keys (prevents timing attacks)
- **Key hashing** for secure API key lookup
- **Input validation** on all API endpoints
- **GORM parameter binding** to prevent SQL injection
- **CORS configuration** for cross-origin request control
- **Rate limiting** middleware (optional)

## Disclaimer

GPT-Load is provided as-is without warranty. While we strive to maintain security, users are responsible for:
- Securing their deployment environment
- Protecting their API keys and credentials
- Complying with applicable AI service provider terms of service
- Implementing appropriate access controls

For questions about security that are not vulnerability reports, please open a GitHub discussion with the `security` tag.
