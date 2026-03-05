# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 1.0.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

If you discover a security vulnerability in roji, please report it through GitHub's private vulnerability reporting feature:

1. Go to the [Security tab](https://github.com/kan/roji/security)
2. Click "Report a vulnerability"
3. Provide details about the vulnerability

Alternatively, you can email the maintainer directly.

### What to include

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

### Response timeline

- **Initial response**: Within 48 hours
- **Status update**: Within 7 days
- **Fix release**: Depends on severity (critical: ASAP, high: 1-2 weeks, medium/low: next release)

### Disclosure policy

- We follow responsible disclosure practices
- Security advisories will be published after a fix is available
- Credit will be given to reporters (unless they prefer to remain anonymous)

## Security Best Practices for Users

When using roji in your development environment:

1. **Keep roji updated** to the latest version
2. **Don't expose roji to the internet** - it's designed for local development only
3. **Trust the CA certificate** only on development machines
4. **Review container labels** before connecting services to the roji network
