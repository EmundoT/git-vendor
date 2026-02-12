# Commit Schema Query Cookbook

Queries using ONLY native git, no custom tooling. The commit
schema turns git history into a queryable semantic database.

For the full protocol spec, see [COMMIT-SCHEMA.md](COMMIT-SCHEMA.md).

---

## Queries Using Tags Alone (works immediately)

### Tag heatmap — what semantic areas have the most churn?

```bash
git log -100 --format='%(trailers:key=Tags,valueonly)' \
  | tr ',' '\n' | sed 's/^ //' | sort | uniq -c | sort -rn
```

### Commit archaeology — history of a domain

```bash
git log v2.0..HEAD \
  --format='%ai|%H|%s|%(trailers:key=Tags,valueonly)|%(trailers:key=Intent,valueonly)' \
  | awk -F'|' '$4 ~ /payments\.tax/'
```

### Semantic changelog — group by domain, not commit type

```bash
git log v1.0..v2.0 --format='%(trailers:key=Tags,valueonly)|%s' \
  | awk -F'|' '{split($1,t,","); for(i in t) print t[i]"|"$2}' | sort
```

### Low-confidence agent commits — review candidates

```bash
git log --format='%H %(trailers:key=Confidence,valueonly)' | grep "low"
```

### Contributions by domain — semantic git shortlog

```bash
git log --format='%aN|%(trailers:key=Tags,valueonly)' \
  | awk -F'|' '{split($2,t,","); for(i in t) print $1"|"t[i]}' \
  | sort | uniq -c | sort -rn
```

### Model quality tracking

```bash
git log --format='%(trailers:key=Model,valueonly)|%(trailers:key=Confidence,valueonly)' \
  | awk -F'|' '$1!=""{c[$1]++;if($2=="low")l[$1]++} END{for(m in c)print m,c[m],l[m]+0}'
```

### Agent context priming — feed LLM before starting work

```bash
git log -20 \
  --format='%H|%s|%(trailers:key=Tags,valueonly)|%(trailers:key=Intent,valueonly)' \
  | awk -F'|' '$3 ~ /auth/'
```

---

## Queries Using Touch (grows as code tag coverage increases)

### Tags/Touch drift — touched security code but didn't tag it

```bash
git log --format='%H|%(trailers:key=Tags,valueonly)|%(trailers:key=Touch,valueonly)' \
  | awk -F'|' '$3 ~ /security/ && $2 !~ /security/'
```

### Semantic bisect — skip commits outside the bug's domain

```bash
git bisect start HEAD v1.0.0
git log HEAD...v1.0.0 --format='%H %(trailers:key=Touch,valueonly)' \
  | grep -v 'auth' | cut -d' ' -f1 \
  | xargs git bisect skip
```

### Impact radius — what domains does a PR touch?

```bash
git log main..feature --format='%(trailers:key=Touch,valueonly)' \
  | tr ',' '\n' | sort -u
```

### Branch health by domain — predict merge conflicts

```bash
git log main..feature --format='%(trailers:key=Touch,valueonly)' \
  | tr ',' '\n' | sort | uniq -c | sort -rn
```

### Tag co-occurrence graph — architectural coupling

```bash
git log -200 --format='%(trailers:key=Touch,valueonly)' \
  | awk -F',' '{for(i=1;i<=NF;i++)for(j=i+1;j<=NF;j++)print $i"|"$j}' \
  | sort | uniq -c | sort -rn
```

### Cross-agent contention — two agents in same domain

```bash
git log --since='1 hour ago' \
  --format='%(trailers:key=Agent-Id,valueonly)|%(trailers:key=Touch,valueonly)' \
  | awk -F'|' '{split($2,t,",");for(i in t)print $1"|"t[i]}' \
  | sort | uniq -c | awk '$1>1'
```

---

## Queries Using Diff Metrics (aggregate analysis at scale)

### Change energy by domain — where is effort going?

```bash
git log -100 \
  --format='%(trailers:key=Touch,valueonly)|%(trailers:key=Diff-Additions,valueonly)' \
  | tr ',' '\n' | sort \
  | awk -F'|' '{sum[$1]+=$2} END{for(k in sum) print sum[k],k}' | sort -rn
```

### Surface-aware release risk

```bash
git log v1.2.0..HEAD --format='%(trailers:key=Diff-Surface,valueonly)' \
  | sort | uniq -c | sort -rn
```

### Vendor impact assessment

```bash
VENDOR_TAGS=$(git log -1 --format='%(trailers:key=Tags,valueonly)' \
  --grep='Vendor-Name: auth-lib')
git log -50 --format='%(trailers:key=Touch,valueonly)' \
  | tr ',' '\n' | grep auth | wc -l
```

### Auto-doc staleness — tag churn since last docs commit

```bash
LAST_DOC=$(git log -1 --format='%H' -- docs/)
git log $LAST_DOC..HEAD --format='%(trailers:key=Touch,valueonly)' \
  | tr ',' '\n' | grep auth | wc -l
```

---

## Full Examples

### Agent feature commit (maximally enriched)

```
feat(auth): add TOTP-based two-factor authentication

Implement TOTP enrollment with QR provisioning, 30-second
verification window, and single-use hashed recovery codes.
Covers #security.mfa requirements from roadmap.

Commit-Schema: agent/v1
Agent-Id: claude-code/kyle-desktop
Model: claude-opus-4-6
Intent: implement 2FA per security roadmap SEC-042
Confidence: high
Refs: SEC-042
Tags: auth.mfa, security, user-management
Touch: auth, auth.session, security, config
Diff-Additions: 342
Diff-Deletions: 12
Diff-Files: 7
Diff-Surface: api
Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
```

Agent note (refs/notes/agent):

```json
{
  "schema": "note/v1",
  "type": "decision",
  "timestamp": "2026-02-09T15:30:00Z",
  "payload": {
    "question": "TOTP vs WebAuthn for initial 2FA",
    "chosen": "TOTP",
    "alternatives": ["WebAuthn", "SMS OTP"],
    "reasoning": "Widest authenticator app support. WebAuthn v2."
  }
}
```

Metrics note (refs/notes/metrics):

```json
{
  "schema": "metrics/v1",
  "tests_run": 52,
  "tests_passed": 52,
  "tests_added": 5,
  "coverage_delta": "+2.1%",
  "duration_ms": 3400
}
```

### Agent vendor update (multi-namespace)

```
chore(vendor): update auth-lib to v3.0.0

Vendor auth-lib v3.0.0 and integrate TOTP enrollment into
existing auth flow.

Commit-Schema: agent/v1
Commit-Schema: vendor/v1
Agent-Id: claude-code/kyle-desktop
Model: claude-opus-4-6
Intent: vendor auth-lib for 2FA support per SEC-042
Tags: vendor.update, security, auth.mfa
Vendor-Name: auth-lib
Vendor-Ref: v3.0.0
Vendor-Commit: 9f8e7d6c5b4a3210fedcba9876543210abcdef01
Vendor-License: MIT
Touch: vendor, auth
Diff-Additions: 1420
Diff-Deletions: 3
Diff-Files: 12
Diff-Surface: internal
Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
```

### Human commit (auto-enriched by hooks)

Human types: `git commit -m "fix off-by-one in tax calc"`

After hooks enrich:

```
fix off-by-one in tax calc

Commit-Schema: manual/v1
Touch: billing.tax, payments
Diff-Additions: 1
Diff-Deletions: 1
Diff-Files: 1
Diff-Surface: internal
```

The human typed 6 words. The hooks added 5 trailers. The commit
is in the tag graph and impact-classified with zero extra effort.

### Multi-vendor commit (positional association)

```
chore(vendor): update auth-lib and crypto-lib

Vendor two libraries together — they share a security
upgrade dependency.

Commit-Schema: vendor/v1
Vendor-Name: auth-lib
Vendor-Ref: v3.1.0
Vendor-Commit: 9f8e7d6c5b4a3210fedcba9876543210abcdef01
Vendor-License: MIT
Vendor-Name: crypto-lib
Vendor-Ref: v2.0.0
Vendor-Commit: 1a2b3c4d5e6f7890abcdef1234567890abcdef01
Vendor-License: Apache-2.0
Tags: vendor.update, security
Touch: vendor, auth, crypto
Diff-Additions: 2100
Diff-Deletions: 45
Diff-Files: 18
Diff-Surface: internal
```

The first `Vendor-Name`/`Vendor-Ref`/`Vendor-Commit` group
describes auth-lib; the second group describes crypto-lib.
Parsers use `TrailerValues("Vendor-Name")` to retrieve all
values in order, associating by position.

### Legacy commit (no hooks installed)

```
fix typo in readme
```

No Commit-Schema. No trailers. Valid forever. Invisible to
tag graph. Parsers return empty trailers.
