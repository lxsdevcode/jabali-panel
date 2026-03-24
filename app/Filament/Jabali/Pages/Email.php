<?php

declare(strict_types=1);

namespace App\Filament\Jabali\Pages;

use App\Filament\Jabali\Pages\Concerns\ManagesAutoresponders;
use App\Filament\Jabali\Pages\Concerns\ManagesEmailLogs;
use App\Filament\Jabali\Pages\Concerns\ManagesEmailSettings;
use App\Filament\Jabali\Pages\Concerns\ManagesEmailSharing;
use App\Filament\Jabali\Pages\Concerns\ManagesForwarders;
use App\Filament\Jabali\Pages\Concerns\ManagesMailboxes;
use App\Models\DnsRecord;
use App\Models\DnsSetting;
use App\Models\Domain;
use App\Models\EmailDomain;
use App\Models\EmailForwarder;
use App\Models\Mailbox;
use App\Services\Agent\InteractsWithAgent;
use App\Services\System\MailRoutingSyncService;
use App\Support\Formatter;
use App\Support\PasswordGenerator;
use App\Support\SafeError;
use App\Support\ServerFacts;
use BackedEnum;
use Exception;
use Filament\Actions\Concerns\InteractsWithActions;
use Filament\Actions\Contracts\HasActions;
use Filament\Forms\Concerns\InteractsWithForms;
use Filament\Forms\Contracts\HasForms;
use Filament\Notifications\Notification;
use Filament\Pages\Page;
use Filament\Schemas\Components\Tabs;
use Filament\Schemas\Components\Tabs\Tab;
use Filament\Schemas\Components\View;
use Filament\Schemas\Schema;
use Filament\Tables\Concerns\InteractsWithTable;
use Filament\Tables\Contracts\HasTable;
use Filament\Tables\Table;
use Illuminate\Contracts\Support\Htmlable;
use Illuminate\Support\Facades\Auth;
use Livewire\Attributes\Url;

class Email extends Page implements HasActions, HasForms, HasTable
{
    use InteractsWithActions;
    use InteractsWithAgent;
    use InteractsWithForms;
    use InteractsWithTable;
    use ManagesAutoresponders;
    use ManagesEmailLogs;
    use ManagesEmailSettings;
    use ManagesEmailSharing;
    use ManagesForwarders;
    use ManagesMailboxes;

    protected static string|BackedEnum|null $navigationIcon = 'heroicon-o-envelope';

    protected static ?int $navigationSort = 3;

    public static function getNavigationLabel(): string
    {
        return __('Email');
    }

    protected string $view = 'filament.jabali.pages.email';

    #[Url(as: 'tab')]
    public ?string $activeTab = 'mailboxes';

    public function getTitle(): string|Htmlable
    {
        return __('Email Management');
    }

    public function mount(): void
    {
        // Normalize the tab value from URL
        $this->activeTab = $this->normalizeTabName($this->activeTab);

        if ($this->activeTab === 'spam') {
            $this->loadSpamSettings();
        }
    }

    public function updatedActiveTab(): void
    {
        $this->activeTab = $this->normalizeTabName($this->activeTab);
        if ($this->activeTab === 'spam') {
            $this->loadSpamSettings();
        }
        $this->resetTable();
    }

    protected function normalizeTabName(?string $tab): string
    {
        // Handle Filament's tab format "tabname::tab" or just "tabname"
        $tab = $tab ?? 'mailboxes';
        if (str_contains($tab, '::')) {
            $tab = explode('::', $tab)[0];
        }

        // Map to valid tab names
        return match ($tab) {
            'mailboxes', 'Mailboxes' => 'mailboxes',
            'forwarders', 'Forwarders' => 'forwarders',
            'autoresponders', 'Autoresponders' => 'autoresponders',
            'catchall', 'catch-all', 'Catch-All' => 'catchall',
            'disclaimer', 'Disclaimer' => 'disclaimer',
            'sharing', 'Sharing', 'shared', 'Shared Folders' => 'sharing',
            'logs', 'Logs' => 'logs',
            'spam', 'Spam' => 'spam',
            default => 'mailboxes',
        };
    }

    protected function getActiveTabIndex(): int
    {
        return match ($this->activeTab) {
            'mailboxes' => 1,
            'forwarders' => 2,
            'autoresponders' => 3,
            'catchall' => 4,
            'disclaimer' => 5,
            'sharing' => 6,
            'logs' => 7,
            'spam' => 8,
            default => 1,
        };
    }

    protected function getForms(): array
    {
        return ['emailForm', 'spamForm'];
    }

    public function emailForm(Schema $schema): Schema
    {
        return $schema->schema([
            Tabs::make(__('Email Sections'))
                ->contained()
                ->livewireProperty('activeTab')
                ->tabs([
                    'mailboxes' => Tab::make(__('Mailboxes'))
                        ->icon('heroicon-o-envelope')
                        ->schema([
                            View::make('filament.jabali.pages.email-tab-table'),
                        ]),
                    'forwarders' => Tab::make(__('Forwarders'))
                        ->icon('heroicon-o-arrow-right')
                        ->schema([
                            View::make('filament.jabali.pages.email-tab-table'),
                        ]),
                    'autoresponders' => Tab::make(__('Autoresponders'))
                        ->icon('heroicon-o-clock')
                        ->schema([
                            View::make('filament.jabali.pages.email-tab-table'),
                        ]),
                    'catchall' => Tab::make(__('Catch-All'))
                        ->icon('heroicon-o-inbox-stack')
                        ->schema([
                            View::make('filament.jabali.pages.email-tab-table'),
                        ]),
                    'disclaimer' => Tab::make(__('Disclaimer'))
                        ->icon('heroicon-o-document-text')
                        ->schema([
                            View::make('filament.jabali.pages.email-tab-table'),
                        ]),
                    'sharing' => Tab::make(__('Shared Folders'))
                        ->icon('heroicon-o-share')
                        ->schema([
                            View::make('filament.jabali.pages.email-tab-table'),
                        ]),
                    'logs' => Tab::make(__('Logs'))
                        ->icon('heroicon-o-document-text')
                        ->schema([
                            View::make('filament.jabali.pages.email-tab-table'),
                        ]),
                    'spam' => Tab::make(__('Spam Settings'))
                        ->icon('heroicon-o-shield-check')
                        ->schema([
                            View::make('filament.jabali.pages.email-tab-spam'),
                        ]),
                ]),
        ]);
    }

    public function setTab(string $tab): void
    {
        $this->activeTab = $this->normalizeTabName($tab);
        if ($this->activeTab === 'spam') {
            $this->loadSpamSettings();
        }
        $this->resetTable();
    }

    public function getUsername(): string
    {
        return Auth::user()->username;
    }

    public function generateSecurePassword(int $length = 16): string
    {
        return PasswordGenerator::generate($length);
    }

    public function table(Table $table): Table
    {
        return match ($this->activeTab) {
            'mailboxes' => $this->mailboxesTable($table),
            'forwarders' => $this->forwardersTable($table),
            'autoresponders' => $this->autorespondersTable($table),
            'catchall' => $this->catchAllTable($table),
            'disclaimer' => $this->disclaimerTable($table),
            'sharing' => $this->sharingTable($table),
            'logs' => $this->emailLogsTable($table),
            'spam' => $this->mailboxesTable($table),
            default => $this->mailboxesTable($table),
        };
    }

    protected function getHeaderActions(): array
    {
        return [
            $this->createMailboxAction(),
            $this->createForwarderAction(),
            $this->createAutoresponderAction(),
            $this->createShareAction(),
            $this->showCredentialsAction(),
        ];
    }

    protected function getOrCreateEmailDomain(Domain $domain): EmailDomain
    {
        $emailDomain = $domain->emailDomain;

        if (! $emailDomain) {
            // Enable email for this domain on the server (also generates DKIM)
            $enableResult = $this->agent()->emailEnableDomain($this->getUsername(), $domain->domain);

            // Create EmailDomain record
            $emailDomain = EmailDomain::create([
                'domain_id' => $domain->id,
                'is_active' => true,
            ]);

            $this->syncMailRouting();

            // Sync Stalwart's DNS records (DKIM, SPF, DMARC, etc.) to BIND
            try {
                $stalwartDns = $enableResult['stalwart_dns_records'] ?? [];

                // Sync Stalwart's recommended DNS records (DKIM, SPF, DMARC, etc.) to BIND
                foreach ($stalwartDns as $rec) {
                    $name = rtrim($rec['name'] ?? '', '.');
                    $type = $rec['type'] ?? '';
                    $content = $rec['content'] ?? '';

                    if (empty($name) || empty($type) || empty($content)) {
                        continue;
                    }

                    // Strip the domain suffix from the name (BIND zone is relative)
                    $name = preg_replace('/\.?'.preg_quote($domain->domain, '/').'\.?$/', '', $name);
                    if (empty($name) || $name === $domain->domain) {
                        $name = '@';
                    }

                    // Update DKIM selector on EmailDomain
                    if (str_contains($name, '_domainkey')) {
                        $selector = explode('.', $name)[0] ?? 'default';
                        $emailDomain->update(['dkim_selector' => $selector]);
                    }

                    DnsRecord::updateOrCreate(
                        [
                            'domain_id' => $domain->id,
                            'name' => $name,
                            'type' => $type,
                        ],
                        [
                            'content' => $content,
                            'ttl' => 3600,
                        ]
                    );
                }

                // Regenerate DNS zone
                $this->regenerateDnsZone($domain);
            } catch (Exception $e) {
                // DNS sync failed, but email can still work
            }

            // Create autoconfig/autodiscover A records and SRV records
            $serverIp = ServerFacts::serverIp('127.0.0.1');
            $defaultTtl = 3600;

            $autoconfigRecords = [
                ['name' => 'autoconfig', 'type' => 'A', 'content' => $serverIp, 'ttl' => $defaultTtl],
                ['name' => 'autodiscover', 'type' => 'A', 'content' => $serverIp, 'ttl' => $defaultTtl],
                ['name' => '_imaps._tcp', 'type' => 'SRV', 'content' => "1 993 mail.{$domain->domain}.", 'ttl' => $defaultTtl, 'priority' => 0],
                ['name' => '_pop3s._tcp', 'type' => 'SRV', 'content' => "1 995 mail.{$domain->domain}.", 'ttl' => $defaultTtl, 'priority' => 0],
                ['name' => '_submission._tcp', 'type' => 'SRV', 'content' => "1 587 mail.{$domain->domain}.", 'ttl' => $defaultTtl, 'priority' => 0],
            ];

            foreach ($autoconfigRecords as $record) {
                DnsRecord::firstOrCreate(
                    [
                        'domain_id' => $domain->id,
                        'name' => $record['name'],
                        'type' => $record['type'],
                    ],
                    $record
                );
            }

            $this->regenerateDnsZone($domain);

            // Issue mail SSL certificate in the background
            \App\Jobs\IssueSslCertificate::dispatch($domain->id, 'mail')->delay(now()->addSeconds(120));
        }

        return $emailDomain;
    }

    protected function syncMailRouting(): void
    {
        try {
            app(MailRoutingSyncService::class)->sync();
        } catch (Exception $e) {
            Notification::make()
                ->title(__('Mail routing sync failed'))
                ->body(SafeError::message($e))
                ->warning()
                ->send();
        }
    }

    protected function regenerateDnsZone(Domain $domain): void
    {
        try {
            $records = DnsRecord::where('domain_id', $domain->id)->get()->toArray();
            $settings = DnsSetting::getAll();
            $hostname = gethostname() ?: 'localhost';
            $serverIp = ServerFacts::serverIp('127.0.0.1');
            $serverIpv6 = $settings['default_ipv6'] ?? null;

            $this->agent()->call('dns.sync_zone', [
                'domain' => $domain->domain,
                'records' => $records,
                'ns1' => $settings['ns1'] ?? "ns1.{$hostname}",
                'ns2' => $settings['ns2'] ?? "ns2.{$hostname}",
                'admin_email' => $settings['admin_email'] ?? "admin.{$hostname}",
                'default_ip' => $settings['default_ip'] ?? $serverIp,
                'default_ipv6' => $serverIpv6,
                'default_ttl' => $settings['default_ttl'] ?? 3600,
            ]);
        } catch (Exception $e) {
            // Log but don't fail - DNS zone regeneration is not critical
        }
    }

    // Helper methods for counts
    public function getMailboxesCount(): int
    {
        return Mailbox::whereHas('emailDomain.domain', fn ($q) => $q->where('user_id', Auth::id()))->count();
    }

    public function getForwardersCount(): int
    {
        return EmailForwarder::whereHas('emailDomain.domain', fn ($q) => $q->where('user_id', Auth::id()))->count();
    }

    public function getCatchAllCount(): int
    {
        return EmailDomain::whereHas('domain', fn ($q) => $q->where('user_id', Auth::id()))->count();
    }

    // Email Usage Stats
    public function getEmailUsageStats(): array
    {
        $domains = EmailDomain::whereHas('domain', fn ($q) => $q->where('user_id', Auth::id()))
            ->with(['mailboxes', 'domain'])
            ->get();

        $totalMailboxes = 0;
        $totalUsed = 0;
        $totalQuota = 0;

        foreach ($domains as $domain) {
            $totalMailboxes += $domain->mailboxes->count();
            $totalUsed += $domain->mailboxes->sum('quota_used_bytes');
            $totalQuota += $domain->mailboxes->sum('quota_bytes');
        }

        return [
            'domains' => $domains->count(),
            'mailboxes' => $totalMailboxes,
            'used_bytes' => $totalUsed,
            'quota_bytes' => $totalQuota,
            'used_formatted' => Formatter::bytes($totalUsed),
            'quota_formatted' => Formatter::bytes($totalQuota),
            'percent' => $totalQuota > 0 ? round(($totalUsed / $totalQuota) * 100, 1) : 0,
        ];
    }
}
