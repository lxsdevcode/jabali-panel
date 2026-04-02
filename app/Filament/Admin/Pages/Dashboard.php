<?php

declare(strict_types=1);

namespace App\Filament\Admin\Pages;

use App\Filament\Admin\Widgets\Dashboard\RecentActivityTable;
use App\Filament\Admin\Widgets\DashboardStatsWidget;
use App\Models\AuditLog;
use App\Models\DnsSetting;
use App\Models\User;
use BackedEnum;
use Filament\Actions\Action;
use Filament\Actions\Concerns\InteractsWithActions;
use Filament\Actions\Contracts\HasActions;
use Filament\Forms\Components\Select;
use Filament\Forms\Components\TextInput;
use Filament\Forms\Concerns\InteractsWithForms;
use Filament\Forms\Contracts\HasForms;
use Filament\Notifications\Notification;
use Filament\Pages\Page;
use Filament\Schemas\Components\EmbeddedTable;
use Filament\Schemas\Components\Grid;
use Filament\Schemas\Components\Section;
use Filament\Schemas\Components\Text;
use Filament\Schemas\Schema;
use Illuminate\Contracts\Support\Htmlable;
use Illuminate\Support\Facades\Cache;
use Illuminate\Support\Str;

class Dashboard extends Page implements HasActions, HasForms
{
    use InteractsWithActions;
    use InteractsWithForms;

    protected static string|BackedEnum|null $navigationIcon = 'heroicon-o-home';

    protected static ?int $navigationSort = 0;

    protected static ?string $slug = 'dashboard';

    protected string $view = 'filament.admin.pages.dashboard';

    public static function getNavigationLabel(): string
    {
        return __('Dashboard');
    }

    public function getTitle(): string|Htmlable
    {
        return __('Dashboard');
    }

    protected function getHeaderWidgets(): array
    {
        return [
            DashboardStatsWidget::class,
        ];
    }

    public function mount(): void
    {
        if (! DnsSetting::get('onboarding_completed', false)) {
            $this->defaultAction = 'onboarding';
        }
    }

    protected function getForms(): array
    {
        return [
            'dashboardForm',
        ];
    }

    public function dashboardForm(Schema $schema): Schema
    {
        return $schema->components([
            // Recent Activity
            Section::make(__('Recent Activity'))
                ->icon('heroicon-o-clock')
                ->schema([
                    EmbeddedTable::make(RecentActivityTable::class),
                ]),
        ]);
    }

    protected function getHeaderActions(): array
    {
        return [
            Action::make('generateLoginLink')
                ->label(__('Login Link'))
                ->icon('heroicon-o-link')
                ->color('info')
                ->modalHeading(__('Generate One-Time Login Link'))
                ->modalDescription(__('Create a shareable login URL that expires after one use.'))
                ->modalWidth('md')
                ->form([
                    Select::make('user_id')
                        ->label(__('User'))
                        ->options(fn () => User::orderBy('username')->pluck('username', 'id')->toArray())
                        ->required()
                        ->searchable()
                        ->live(),
                    Select::make('panel')
                        ->label(__('Panel'))
                        ->options([
                            'admin' => __('Admin Panel'),
                            'user' => __('User Panel'),
                        ])
                        ->default('admin')
                        ->required(),
                    Select::make('ttl')
                        ->label(__('Expires in'))
                        ->options([
                            '15' => __('15 minutes'),
                            '30' => __('30 minutes'),
                            '60' => __('1 hour'),
                        ])
                        ->default('15')
                        ->required(),
                ])
                ->modalSubmitActionLabel(__('Generate'))
                ->action(function (array $data): void {
                    $user = User::find($data['user_id']);
                    if (! $user) {
                        Notification::make()->title(__('User not found'))->danger()->send();

                        return;
                    }

                    $ttl = (int) $data['ttl'];
                    $panel = $data['panel'];
                    $token = Str::random(64);
                    $cacheKey = 'login_token:'.hash('sha256', $token);

                    Cache::put($cacheKey, [
                        'user_id' => $user->id,
                        'panel' => $panel,
                    ], now()->addMinutes($ttl));

                    $hostname = config('jabali.panel.hostname') ?: request()->getHost();
                    $port = config('jabali.panel.port', 8443);
                    $url = "https://{$hostname}:{$port}/auto-login?token={$token}";

                    AuditLog::log(
                        'login_token_generated',
                        'auth',
                        "Login link generated for {$user->username} ({$panel} panel, {$ttl} min)",
                        'user',
                        $user->id,
                        $user->username,
                        ['panel' => $panel, 'ttl' => $ttl]
                    );

                    Notification::make()
                        ->title(__('Login link generated'))
                        ->body($url)
                        ->success()
                        ->persistent()
                        ->send();
                }),

            Action::make('refresh')
                ->label(__('Refresh'))
                ->icon('heroicon-o-arrow-path')
                ->color('gray')
                ->action(fn () => $this->redirect(static::getUrl())),

            Action::make('onboarding')->modalCancelActionLabel(__('Maybe later'))
                ->label(__('Setup Wizard'))
                ->icon('heroicon-o-sparkles')

                ->modalHeading(__('Welcome to Jabali!'))
                ->modalDescription(__('Let\'s get your server control panel set up.'))
                ->modalWidth('2xl')
                ->fillForm(function (): array {
                    $savedRecipients = trim((string) DnsSetting::get('admin_email_recipients', ''));
                    $savedPrimaryEmail = $savedRecipients === '' ? '' : trim(explode(',', $savedRecipients)[0]);

                    return [
                        'admin_email' => $savedPrimaryEmail,
                    ];
                })
                ->form([
                    Section::make(__('Next Steps'))
                        ->description(__('Here is a quick setup path to get your first site online.'))
                        ->icon('heroicon-o-check-circle')
                        ->iconColor('info')
                        ->collapsed(false)
                        ->collapsible(false)
                        ->compact()
                        ->schema([
                            Grid::make(['default' => 1, 'md' => 2])
                                ->schema([
                                    Section::make(__('1. Configure Server Settings'))
                                        ->description(__('Set hostname, DNS, email, storage, and PHP defaults.'))
                                        ->icon('heroicon-o-cog-6-tooth')
                                        ->iconColor('info')
                                        ->collapsed(false)
                                        ->collapsible(false)
                                        ->compact(),
                                    Section::make(__('2. Create a Hosting Package'))
                                        ->description(__('Define limits and features for your plans.'))
                                        ->icon('heroicon-o-cube')
                                        ->iconColor('info')
                                        ->collapsed(false)
                                        ->collapsible(false)
                                        ->compact(),
                                    Section::make(__('3. Create a User'))
                                        ->description(__('Assign the hosting package and set credentials.'))
                                        ->icon('heroicon-o-user-plus')
                                        ->iconColor('info')
                                        ->collapsed(false)
                                        ->collapsible(false)
                                        ->compact(),
                                    Section::make(__('4. Add a Domain'))
                                        ->description(__('Issue SSL and deploy your site files.'))
                                        ->icon('heroicon-o-globe-alt')
                                        ->iconColor('info')
                                        ->collapsed(false)
                                        ->collapsible(false)
                                        ->compact(),
                                ]),
                            Text::make(__('Optional: review Services and Server Status to confirm everything is healthy.'))
                                ->color('gray'),
                        ]),
                    TextInput::make('admin_email')
                        ->label(__('Your Email Address'))
                        ->helperText(__('Enter your email to receive important server notifications.'))
                        ->email()
                        ->placeholder(__('admin@example.com')),
                ])
                ->modalSubmitActionLabel(__('Save and close'))
                ->action(function (array $data): void {
                    $adminEmail = trim((string) ($data['admin_email'] ?? ''));

                    if ($adminEmail !== '') {
                        DnsSetting::set('admin_email_recipients', $adminEmail);
                    }

                    DnsSetting::set('onboarding_completed', '1');
                    DnsSetting::clearCache();

                    Notification::make()
                        ->title(__('Setup saved'))
                        ->body(__('Your notification email has been updated.'))
                        ->success()
                        ->send();
                }),
        ];
    }
}
