# Security Policy

## Reporting a Vulnerability

Please report any suspected vulnerabilities using GitHub's private
vulnerability reporting feature. You may also reach me by email at the author
email for my commits in this repository.

While I will make every effort to address vulnerabilities in the library
promptly, I can't offer any timeline or guarantee for response or resolution at
this time.

### Scope

Crashes, unbounded resource utilization, and other security issues in the
library when parsing patches or applying previously parsed patches are in-scope
for vulnerability reporting.

Issues that originate outside of the library from clients handling untrusted
patches are generally out of scope, as are issues that result from clients
applying manually-constructed rather than parsed patches. These types of
problems may still be worth reporting as regular issues, rather than as
vulnerabilities.

## Supported Versions

Only the latest tagged release receives security updates. In exceptional cases
or if there have been significant breaking changes, I may backport security
fixes to earlier releases on a case-by-case basis.

This policy may change after a 1.x release.
