<?php

declare(strict_types=1);

namespace App\Services;

class NginxDirectiveValidator
{
    /**
     * Directives blocked for all users (security risk).
     */
    private const BLOCKED_DIRECTIVES = [
        'include',
        'alias',
        'root',
        'lua_',
        'content_by_lua',
        'access_by_lua',
        'rewrite_by_lua',
        'load_module',
        'ssl_certificate',
        'ssl_certificate_key',
        'ssl_trusted_certificate',
        'perl_',
        'js_',
    ];

    /**
     * Directives allowed for regular users.
     */
    private const ALLOWED_DIRECTIVES = [
        'rewrite',
        'return',
        'try_files',
        'add_header',
        'more_set_headers',
        'location',
        'proxy_pass',
        'proxy_set_header',
        'proxy_http_version',
        'proxy_read_timeout',
        'proxy_connect_timeout',
        'proxy_send_timeout',
        'limit_req',
        'limit_req_zone',
        'allow',
        'deny',
        'fastcgi_param',
        'client_max_body_size',
        'expires',
        'access_log',
        'if',
        'set',
    ];

    /**
     * Validate directives for a regular user (restricted).
     *
     * @return array{valid: bool, error: ?string}
     */
    public static function validateForUser(string $directives): array
    {
        if (trim($directives) === '') {
            return ['valid' => true, 'error' => null];
        }

        // Check for blocked directives first
        foreach (self::BLOCKED_DIRECTIVES as $blocked) {
            if (preg_match('/(?:^|\s)'.preg_quote($blocked, '/').'\b/mi', $directives)) {
                return [
                    'valid' => false,
                    'error' => "The directive '{$blocked}' is not allowed for security reasons.",
                ];
            }
        }

        // Extract top-level directive names (first word of each non-comment, non-brace line)
        $lines = explode("\n", $directives);
        foreach ($lines as $lineNum => $line) {
            $trimmed = trim($line);

            // Skip empty lines, comments, closing braces
            if ($trimmed === '' || str_starts_with($trimmed, '#') || $trimmed === '}') {
                continue;
            }

            // Lines inside blocks (indented) — extract the directive name
            if (preg_match('/^([a-z_]+)/', $trimmed, $matches)) {
                $directive = $matches[1];

                // Check if it's in the allowed list
                $allowed = false;
                foreach (self::ALLOWED_DIRECTIVES as $ok) {
                    if ($directive === $ok || str_starts_with($directive, $ok)) {
                        $allowed = true;
                        break;
                    }
                }

                if (! $allowed) {
                    $displayLine = $lineNum + 1;

                    return [
                        'valid' => false,
                        'error' => "Directive '{$directive}' on line {$displayLine} is not allowed. Contact your administrator for advanced configuration.",
                    ];
                }
            }
        }

        // Check for path traversal attempts in any string values
        if (preg_match('/\.\.\/?/', $directives)) {
            return ['valid' => false, 'error' => 'Path traversal (../) is not allowed.'];
        }

        return ['valid' => true, 'error' => null];
    }

    /**
     * Validate directives for an admin (only blocked directives checked).
     *
     * @return array{valid: bool, error: ?string}
     */
    public static function validateForAdmin(string $directives): array
    {
        if (trim($directives) === '') {
            return ['valid' => true, 'error' => null];
        }

        // Admins can use anything except the most dangerous directives
        foreach (['load_module', 'lua_', 'perl_', 'js_'] as $blocked) {
            if (preg_match('/(?:^|\s)'.preg_quote($blocked, '/').'\b/mi', $directives)) {
                return [
                    'valid' => false,
                    'error' => "The directive '{$blocked}' is blocked even for admins.",
                ];
            }
        }

        return ['valid' => true, 'error' => null];
    }
}
