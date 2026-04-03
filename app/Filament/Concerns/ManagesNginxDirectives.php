<?php

declare(strict_types=1);

namespace App\Filament\Concerns;

use App\Models\Domain;
use App\Services\Agent\AgentClient;
use App\Services\NginxDirectiveValidator;
use Filament\Actions\Action;
use Filament\Forms\Components\Placeholder;
use Filament\Forms\Components\Repeater;
use Filament\Forms\Components\Select;
use Filament\Forms\Components\Textarea;
use Filament\Forms\Components\TextInput;
use Filament\Forms\Components\Toggle;
use Filament\Notifications\Notification;
use Filament\Schemas\Components\Tabs;
use Filament\Schemas\Components\Tabs\Tab;
use Filament\Schemas\Components\Utilities\Get;
use Illuminate\Support\HtmlString;

trait ManagesNginxDirectives
{
    public bool $nginxTestPassed = false;

    public function nginxDirectivesAction(): Action
    {
        $isAdmin = auth()->user()?->is_admin;

        return Action::make('nginxDirectives')
            ->label(__('Nginx Directives'))
            ->icon('heroicon-o-code-bracket')
            ->color('warning')
            ->form([
                Placeholder::make('warning')
                    ->hiddenLabel()
                    ->content(new HtmlString(
                        '<div class="flex gap-3 rounded-lg bg-danger-50 p-4 text-sm text-danger-700 dark:bg-danger-950/50 dark:text-danger-400">'
                        .'<svg class="mt-0.5 h-5 w-5 shrink-0" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M8.485 2.495c.673-1.167 2.357-1.167 3.03 0l6.28 10.875c.673 1.167-.168 2.625-1.516 2.625H3.72c-1.347 0-2.189-1.458-1.515-2.625L8.485 2.495zM10 5a.75.75 0 01.75.75v3.5a.75.75 0 01-1.5 0v-3.5A.75.75 0 0110 5zm0 9a1 1 0 100-2 1 1 0 000 2z" clip-rule="evenodd"/></svg>'
                        .'<div>'.__('Incorrect directives can break your website. Changes are tested with nginx before applying, but you are responsible for ensuring your configuration is correct.').'</div>'
                        .'</div>'
                    )),
                Tabs::make('directives')
                    ->tabs([
                        Tab::make(__('Rule Builder'))
                            ->icon('heroicon-o-wrench-screwdriver')
                            ->schema([
                                Repeater::make('rules')
                                    ->label(__('Add rules using the form below. They will be converted to nginx directives automatically.'))
                                    ->live()
                                    ->afterStateUpdated(fn (?array $state) => $this->nginxTestPassed = empty($state))
                                    ->schema([
                                        Select::make('type')
                                            ->label(__('Type'))
                                            ->options([
                                                'redirect' => __('Redirect'),
                                                'header' => __('Custom Header'),
                                                'rewrite' => __('Rewrite Rule'),
                                                'proxy' => __('Proxy Pass'),
                                                'ip_access' => __('IP Access Control'),
                                                'php_value' => __('PHP Setting'),
                                                'client_max_body' => __('Max Upload Size'),
                                            ])
                                            ->required()
                                            ->live(),

                                        // Redirect fields
                                        TextInput::make('source')
                                            ->label(__('Source Path'))
                                            ->placeholder('/old-page')
                                            ->required(fn (Get $get) => in_array($get('type'), ['redirect', 'rewrite', 'proxy']))
                                            ->regex('/^\//')
                                            ->validationMessages(['regex' => __('Must start with /')])
                                            ->visible(fn (Get $get) => in_array($get('type'), ['redirect', 'rewrite', 'proxy', 'ip_access'])),
                                        TextInput::make('destination')
                                            ->label(__('Destination'))
                                            ->placeholder('https://example.com/new-page')
                                            ->required(fn (Get $get) => in_array($get('type'), ['redirect', 'proxy']))
                                            ->regex('/^(https?:\/\/|\/)\S+$/')
                                            ->validationMessages(['regex' => __('Must be a URL or path starting with /')])
                                            ->visible(fn (Get $get) => in_array($get('type'), ['redirect', 'rewrite', 'proxy'])),
                                        Select::make('redirect_type')
                                            ->label(__('Redirect Type'))
                                            ->options(['301' => '301 Permanent', '302' => '302 Temporary'])
                                            ->default('301')
                                            ->visible(fn (Get $get) => $get('type') === 'redirect'),

                                        // Header fields
                                        Select::make('header_name')
                                            ->label(__('Header'))
                                            ->required(fn (Get $get) => $get('type') === 'header')
                                            ->options([
                                                'X-Frame-Options' => 'X-Frame-Options',
                                                'X-Content-Type-Options' => 'X-Content-Type-Options',
                                                'Strict-Transport-Security' => 'Strict-Transport-Security',
                                                'Content-Security-Policy' => 'Content-Security-Policy',
                                                'Referrer-Policy' => 'Referrer-Policy',
                                                'Permissions-Policy' => 'Permissions-Policy',
                                                'custom' => __('Custom'),
                                            ])
                                            ->visible(fn (Get $get) => $get('type') === 'header'),
                                        TextInput::make('header_custom_name')
                                            ->label(__('Header Name'))
                                            ->required(fn (Get $get) => $get('type') === 'header' && $get('header_name') === 'custom')
                                            ->regex('/^[A-Za-z][A-Za-z0-9-]*$/')
                                            ->validationMessages(['regex' => __('Invalid header name (letters, digits, hyphens only)')])
                                            ->visible(fn (Get $get) => $get('type') === 'header' && $get('header_name') === 'custom'),
                                        TextInput::make('header_value')
                                            ->label(__('Value'))
                                            ->required(fn (Get $get) => $get('type') === 'header')
                                            ->maxLength(1024)
                                            ->visible(fn (Get $get) => $get('type') === 'header'),

                                        // Rewrite fields
                                        Select::make('rewrite_flag')
                                            ->label(__('Flag'))
                                            ->options(['last' => 'last', 'break' => 'break', 'permanent' => 'permanent', 'redirect' => 'redirect'])
                                            ->default('last')
                                            ->visible(fn (Get $get) => $get('type') === 'rewrite'),

                                        // Proxy fields
                                        Toggle::make('proxy_websocket')
                                            ->label(__('WebSocket Support'))
                                            ->visible(fn (Get $get) => $get('type') === 'proxy'),

                                        // IP Access fields
                                        Select::make('ip_action')
                                            ->label(__('Action'))
                                            ->required(fn (Get $get) => $get('type') === 'ip_access')
                                            ->options(['allow' => __('Allow'), 'deny' => __('Deny')])
                                            ->visible(fn (Get $get) => $get('type') === 'ip_access'),
                                        TextInput::make('ip_address')
                                            ->label(__('IP / CIDR'))
                                            ->placeholder('1.2.3.4 or 10.0.0.0/8')
                                            ->required(fn (Get $get) => $get('type') === 'ip_access')
                                            ->regex('/^(\d{1,3}\.){3}\d{1,3}(\/\d{1,2})?$/')
                                            ->validationMessages(['regex' => __('Must be a valid IPv4 address or CIDR (e.g. 10.0.0.1 or 10.0.0.0/24)')])
                                            ->visible(fn (Get $get) => $get('type') === 'ip_access'),

                                        // PHP value fields
                                        Select::make('php_setting')
                                            ->label(__('Setting'))
                                            ->required(fn (Get $get) => $get('type') === 'php_value')
                                            ->options([
                                                'memory_limit' => 'memory_limit',
                                                'upload_max_filesize' => 'upload_max_filesize',
                                                'post_max_size' => 'post_max_size',
                                                'max_execution_time' => 'max_execution_time',
                                                'max_input_vars' => 'max_input_vars',
                                            ])
                                            ->live()
                                            ->visible(fn (Get $get) => $get('type') === 'php_value'),
                                        TextInput::make('php_value')
                                            ->label(__('Value'))
                                            ->placeholder(fn (Get $get) => in_array($get('php_setting'), ['max_execution_time', 'max_input_vars']) ? '300' : '512M')
                                            ->required(fn (Get $get) => $get('type') === 'php_value')
                                            ->regex(fn (Get $get) => in_array($get('php_setting'), ['max_execution_time', 'max_input_vars'])
                                                ? '/^\d+$/'
                                                : '/^\d+[KkMmGg]$/')
                                            ->validationMessages(['regex' => __('Must be a valid PHP value (e.g. 512M, 1G, 300)')])
                                            ->visible(fn (Get $get) => $get('type') === 'php_value'),

                                        // Max body size
                                        TextInput::make('max_body_size')
                                            ->label(__('Max Size'))
                                            ->placeholder('100m')
                                            ->required(fn (Get $get) => $get('type') === 'client_max_body')
                                            ->regex('/^\d+[KkMmGg]$/')
                                            ->validationMessages(['regex' => __('Must be a size value (e.g. 100m, 1G)')])
                                            ->visible(fn (Get $get) => $get('type') === 'client_max_body'),
                                    ])
                                    ->columns(2)
                                    ->defaultItems(0)
                                    ->addActionLabel(__('Add Rule'))
                                    ->addAction(fn (Action $action) => $action->after(fn () => $this->nginxTestPassed = false))
                                    ->deleteAction(fn (Action $action) => $action->after(fn (Repeater $component) => $this->nginxTestPassed = empty($component->getState())))
                                    ->reorderable(),
                            ]),
                        Tab::make(__('Raw Directives'))
                            ->icon('heroicon-o-code-bracket')
                            ->schema([
                                Textarea::make('raw_directives')
                                    ->label('')
                                    ->rows(12)
                                    ->placeholder("# Example:\nrewrite ^/old$ /new permanent;\nadd_header X-Frame-Options \"DENY\" always;")
                                    ->helperText($isAdmin
                                        ? __('Admin mode: most directives allowed.')
                                        : __('Restricted to safe directives (rewrite, add_header, proxy_pass, etc.). Dangerous directives are blocked.'))
                                    ->live(onBlur: true)
                                    ->afterStateUpdated(fn (?string $state) => $this->nginxTestPassed = empty(trim($state ?? ''))),
                            ]),
                    ]),
            ])
            ->modalSubmitAction(fn (Action $action) => $action
                ->label(fn () => $this->nginxTestPassed ? __('Apply') : __('Test Configuration'))
                ->icon(fn () => $this->nginxTestPassed ? 'heroicon-o-check' : 'heroicon-o-beaker')
                ->color(fn () => $this->nginxTestPassed ? 'success' : 'info')
            )
            ->fillForm(function (Domain $record): array {
                $hasContent = ! empty($record->custom_nginx_rules) || ! empty($record->custom_nginx_directives);
                $this->nginxTestPassed = ! $hasContent;

                return [
                    'rules' => $record->custom_nginx_rules ?? [],
                    'raw_directives' => $record->custom_nginx_directives ?? '',
                ];
            })
            ->action(function (array $data, Domain $record): void {
                $rules = $data['rules'] ?? [];
                $rawDirectives = trim($data['raw_directives'] ?? '');

                $generated = self::generateDirectives($rules);
                $combined = trim($generated."\n\n".$rawDirectives);

                // Validate
                $isAdmin = auth()->user()?->is_admin;
                $validation = $isAdmin
                    ? NginxDirectiveValidator::validateForAdmin($combined)
                    : NginxDirectiveValidator::validateForUser($combined);

                if (! $validation['valid']) {
                    Notification::make()
                        ->title(__('Validation Error'))
                        ->body($validation['error'])
                        ->danger()
                        ->send();
                    $this->nginxTestPassed = false;

                    $this->halt();

                    return;
                }

                $agent = app(AgentClient::class);

                // Empty = clear all custom directives (no test needed)
                if (trim($combined) === '') {
                    $agent->call('domain.apply_custom_directives', [
                        'domain' => $record->domain,
                        'directives' => '',
                    ]);

                    $record->update([
                        'custom_nginx_directives' => null,
                        'custom_nginx_rules' => null,
                    ]);

                    $this->nginxTestPassed = false;

                    Notification::make()
                        ->title(__('Custom directives cleared'))
                        ->success()
                        ->send();

                    return;
                }

                // Step 1: Test first
                if (! $this->nginxTestPassed) {

                    $result = $agent->call('domain.test_custom_directives', [
                        'domain' => $record->domain,
                        'directives' => $combined,
                    ]);

                    if (! $result->success) {
                        Notification::make()
                            ->title(__('Test Failed'))
                            ->body($result->error ?? __('Nginx config test failed'))
                            ->danger()
                            ->persistent()
                            ->send();
                    } else {
                        $this->nginxTestPassed = true;
                        Notification::make()
                            ->title(__('Test Passed'))
                            ->body(__('Click Apply to save the changes.'))
                            ->success()
                            ->send();
                    }

                    $this->halt();

                    return;
                }

                // Step 2: Apply (tests again as safety net)
                $result = $agent->call('domain.apply_custom_directives', [
                    'domain' => $record->domain,
                    'directives' => $combined,
                ]);

                if (! $result->success) {
                    Notification::make()
                        ->title(__('Nginx Config Error'))
                        ->body($result->error ?? __('Failed to apply directives'))
                        ->danger()
                        ->persistent()
                        ->send();
                    $this->nginxTestPassed = false;

                    $this->halt();

                    return;
                }

                $record->update([
                    'custom_nginx_directives' => $rawDirectives ?: null,
                    'custom_nginx_rules' => ! empty($rules) ? $rules : null,
                ]);

                $this->nginxTestPassed = false;

                Notification::make()
                    ->title(__('Nginx directives applied'))
                    ->success()
                    ->send();
            });
    }

    private static function generateDirectives(array $rules): string
    {
        $lines = [];

        foreach ($rules as $rule) {
            $type = $rule['type'] ?? '';

            match ($type) {
                'redirect' => $lines[] = self::buildRedirect($rule),
                'header' => $lines[] = self::buildHeader($rule),
                'rewrite' => $lines[] = self::buildRewrite($rule),
                'proxy' => $lines[] = self::buildProxy($rule),
                'ip_access' => $lines[] = self::buildIpAccess($rule),
                'php_value' => $lines[] = self::buildPhpValue($rule),
                'client_max_body' => $lines[] = self::buildMaxBody($rule),
                default => null,
            };
        }

        return implode("\n", array_filter($lines));
    }

    private static function buildRedirect(array $rule): string
    {
        $source = $rule['source'] ?? '';
        $dest = $rule['destination'] ?? '';
        $type = ($rule['redirect_type'] ?? '301') === '301' ? 'permanent' : 'redirect';

        if (empty($source) || empty($dest)) {
            return '';
        }

        return "rewrite ^{$source}\$ {$dest} {$type};";
    }

    private static function buildHeader(array $rule): string
    {
        $name = $rule['header_name'] ?? '';
        if ($name === 'custom') {
            $name = $rule['header_custom_name'] ?? '';
        }
        $value = $rule['header_value'] ?? '';

        if (empty($name) || empty($value)) {
            return '';
        }

        return "add_header {$name} \"{$value}\" always;";
    }

    private static function buildRewrite(array $rule): string
    {
        $source = $rule['source'] ?? '';
        $dest = $rule['destination'] ?? '';
        $flag = $rule['rewrite_flag'] ?? 'last';

        if (empty($source) || empty($dest)) {
            return '';
        }

        return "rewrite {$source} {$dest} {$flag};";
    }

    private static function buildProxy(array $rule): string
    {
        $path = $rule['source'] ?? '';
        $upstream = $rule['destination'] ?? '';
        $ws = $rule['proxy_websocket'] ?? false;

        if (empty($path) || empty($upstream)) {
            return '';
        }

        $block = "location {$path} {\n";
        $block .= "    proxy_pass {$upstream};\n";
        $block .= "    proxy_set_header Host \$host;\n";
        $block .= "    proxy_set_header X-Real-IP \$remote_addr;\n";
        $block .= "    proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;\n";
        $block .= "    proxy_set_header X-Forwarded-Proto \$scheme;\n";
        if ($ws) {
            $block .= "    proxy_http_version 1.1;\n";
            $block .= "    proxy_set_header Upgrade \$http_upgrade;\n";
            $block .= "    proxy_set_header Connection \"upgrade\";\n";
        }
        $block .= '}';

        return $block;
    }

    private static function buildIpAccess(array $rule): string
    {
        $path = $rule['source'] ?? '/';
        $action = $rule['ip_action'] ?? 'deny';
        $ip = $rule['ip_address'] ?? '';

        if (empty($ip)) {
            return '';
        }

        $opposite = $action === 'allow' ? 'deny all' : 'allow all';

        return "location {$path} {\n    {$action} {$ip};\n    {$opposite};\n}";
    }

    private static function buildPhpValue(array $rule): string
    {
        $setting = $rule['php_setting'] ?? '';
        $value = $rule['php_value'] ?? '';

        if (empty($setting) || empty($value)) {
            return '';
        }

        return "fastcgi_param PHP_VALUE \"{$setting}={$value}\";";
    }

    private static function buildMaxBody(array $rule): string
    {
        $size = $rule['max_body_size'] ?? '';

        if (empty($size)) {
            return '';
        }

        return "client_max_body_size {$size};";
    }
}
