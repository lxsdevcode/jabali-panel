#!/usr/bin/env bash
# jabali-hostname.sh — JAB-213 free-hostname client, sourced by install.sh.
#
# Provides the interactive register->code->claim flow that gives a box with no
# domain a publicly-resolvable `<ip>.jabalihosted.com` hostname + TLS, the way
# cPanel gives every server a free `*.cprapid.com`. The service (api.jabali
# hosted.com) derives the label from the box's observed public IP, so the box
# never chooses it.
#
# Contract with install.sh:
#   jh_free_hostname_flow  -> on success echoes the claimed FQDN on stdout and
#                             writes /etc/jabali-panel/hostname.env (0600);
#                             returns non-zero (with a reason on stderr) when
#                             the operator should fall back to the manual
#                             hostname prompt.
#
# SECURITY: the bearer token returned by /v1/claim must NEVER hit the install
# log (which wraps every command's output). We parse the JSON with python3 and
# emit only the FQDN on stdout; the token goes straight into hostname.env.
# The email is stored for operational contact only.

JH_API="${JABALI_HOSTNAME_API:-https://api.jabalihosted.com}"
JH_TOKEN_FILE=/etc/jabali-panel/hostname.env

# jh_post <path> <json> -> prints `<body>\n<http_code>` on stdout. The caller
# splits it locally (a global can't cross the command-substitution subshell
# boundary, which is why this returns both inline). On a transport failure the
# code is 000 and the body empty.
jh_post() {
  local path="$1" body="$2"
  curl -sS --max-time 30 -w $'\n%{http_code}' \
    -H 'Content-Type: application/json' \
    -d "$body" "${JH_API}${path}" 2>/dev/null || printf '\n000'
}

# jh_field <json> <key> -> value (empty if absent). python3 is an install.sh dep.
jh_field() { printf '%s' "$1" | python3 -c 'import json,sys; print(json.load(sys.stdin).get(sys.argv[1],""))' "$2" 2>/dev/null; }

# jh_free_hostname_flow <input_fd>  (fd must be an open TTY for the code prompt)
jh_free_hostname_flow() {
  local fd="$1" email code body fqdn label token

  if [[ -z "$fd" ]]; then
    echo "free hostname needs an interactive terminal (email + code)" >&2
    return 1
  fi

  # --- email ---
  {
    printf '\n'
    printf 'Free Jabali hostname\n'
    printf '  You will get a public hostname like 203-0-113-7.jabalihosted.com\n'
    printf '  with automatic DNS + TLS. We email a one-time code to verify the\n'
    printf '  address; it is stored only so we can contact you about the hostname.\n\n'
  } > /dev/tty 2>/dev/null || true

  local email_re='^[^@[:space:]]+@[^@[:space:]]+\.[^@[:space:]]+$'
  while true; do
    printf 'Email for verification: ' > /dev/tty 2>/dev/null || printf 'Email for verification: '
    read -r -u "$fd" email || true
    [[ "$email" =~ $email_re ]] && break
    echo "  please enter a valid email address" > /dev/tty 2>/dev/null || true
  done

  # --- register (one retry on transient) ---
  local resp jcode attempt
  for attempt in 1 2; do
    resp="$(jh_post /v1/register "{\"email\":\"${email}\"}")"
    jcode="${resp##*$'\n'}"; body="${resp%$'\n'*}"
    case "$jcode" in
      200) break ;;
      429) echo "  a code was just sent — wait a minute and re-run" >&2; return 1 ;;
      000|5*) [[ $attempt == 1 ]] && { sleep 3; continue; }
             echo "  hostname service unreachable ($jcode) — using a manual hostname" >&2; return 1 ;;
      *)   echo "  could not send a code ($(jh_field "$body" error)) — using a manual hostname" >&2; return 1 ;;
    esac
  done
  printf '  code sent to %s\n' "$email" > /dev/tty 2>/dev/null || true

  # --- code -> claim ---
  local tries
  for tries in 1 2 3; do
    printf 'Enter the 6-digit code: ' > /dev/tty 2>/dev/null || printf 'Enter the 6-digit code: '
    read -r -u "$fd" code || true
    code="${code//[^0-9]/}"
    resp="$(jh_post /v1/claim "{\"email\":\"${email}\",\"code\":\"${code}\"}")"
    jcode="${resp##*$'\n'}"; body="${resp%$'\n'*}"
    case "$jcode" in
      200) break ;;
      403) echo "  wrong or expired code, try again" > /dev/tty 2>/dev/null || true
           [[ $tries == 3 ]] && { echo "  no valid code entered — using a manual hostname" >&2; return 1; }
           continue ;;
      429) echo "  too many attempts — re-run the installer to get a fresh code" >&2; return 1 ;;
      422) # bad_source: CGNAT/private/bogon public IP — free hostname impossible
           echo "  this server's public IP can't take a free hostname ($(jh_field "$body" message))" >&2
           echo "  falling back to a manual hostname" >&2; return 1 ;;
      *)   echo "  claim failed ($jcode) — using a manual hostname" >&2; return 1 ;;
    esac
  done

  fqdn="$(jh_field "$body" fqdn)"
  label="$(jh_field "$body" label)"
  token="$(jh_field "$body" token)"
  if [[ -z "$fqdn" || -z "$token" ]]; then
    echo "  malformed claim response — using a manual hostname" >&2; return 1
  fi

  # Persist token OUT of band (never logged). Parent dir /etc/jabali-panel
  # stays 0755 (the /etc/jabali 0755 SSH-lockout scar generalizes); only the
  # file is 0600.
  umask 077
  {
    printf 'LABEL=%s\n' "$label"
    printf 'FQDN=%s\n' "$fqdn"
    printf 'EMAIL=%s\n' "$email"
    printf 'TOKEN=%s\n' "$token"
    printf 'API=%s\n' "$JH_API"
  } > "$JH_TOKEN_FILE"
  chmod 0600 "$JH_TOKEN_FILE"

  printf '%s' "$fqdn"   # <-- the only stdout; install.sh captures it
  return 0
}
