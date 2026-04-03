# One-Time Login Tokens

Generate shareable login URLs that expire after one use. Useful for granting temporary access to developers or support staff.

## How It Works

1. A random 64-character token is generated
2. SHA-256 hash of the token + user/panel info stored in cache with TTL
3. URL format: `https://hostname:8443/auto-login?token=TOKEN`
4. On first use, `Cache::pull()` retrieves and deletes the token (one-time)
5. User is logged into the correct guard and redirected to the appropriate panel

## CLI

```bash
jabali login token                    # Interactive user selection
jabali login token --user=admin       # Specific user
jabali login token --ttl=30           # 30 minute expiry (default: 15)
jabali login token --panel=user       # Force user panel
jabali login token --json             # JSON output
```

## Admin Dashboard

The **"Login Link"** button in the admin dashboard header opens a modal with:

- User selector (all users)
- Panel choice (admin or user)
- TTL selector (15m, 30m, 1h)

After generating, a modal shows the URL with a copy button.

## User Dashboard

The **"Support Access"** button lets users generate a token for their own account:

- User panel only (no admin access)
- TTL selector
- URL shown in a copyable modal

Users can share this link with a developer for temporary access to their hosting panel.

## Security

- Tokens are SHA-256 hashed in cache (raw token only in URL)
- One-time use via `Cache::pull()`
- No IP binding (tokens are meant to be shared)
- All token generation is audit logged
- Regular users can only generate tokens for themselves
