<?php

declare(strict_types=1);

namespace App\Filament\Admin\Pages;

use Filament\Facades\Filament;
use Filament\Forms\Components\TextInput;
use Filament\Forms\Concerns\InteractsWithForms;
use Filament\Forms\Contracts\HasForms;
use Filament\Notifications\Notification;
use Filament\Pages\SimplePage;
use Filament\Schemas\Schema;
use Illuminate\Contracts\Support\Htmlable;
use Illuminate\Support\Facades\Auth;
use Laravel\Fortify\Actions\EnableTwoFactorAuthentication;
use Laravel\Fortify\Contracts\TwoFactorAuthenticationProvider;

class TwoFactorSetup extends SimplePage implements HasForms
{
    use InteractsWithForms;

    protected static string $routePath = '/two-factor-setup';

    protected static bool $shouldRegisterNavigation = false;

    protected string $view = 'filament.admin.pages.two-factor-setup';

    public ?array $data = [];

    public bool $secretGenerated = false;

    public ?string $qrCodeSvg = null;

    public ?string $setupSecret = null;

    public function mount(): void
    {
        $user = Filament::auth()->user();

        if (! $user || ! $user->is_admin) {
            $this->redirect(Filament::getPanel('admin')->getLoginUrl());

            return;
        }

        // If 2FA is already configured, go to dashboard
        if ($user->two_factor_secret && $user->two_factor_confirmed_at) {
            $this->redirect(Filament::getPanel('admin')->getUrl());

            return;
        }

        // Generate the 2FA secret if not already done
        if (! $user->two_factor_secret) {
            app(EnableTwoFactorAuthentication::class)($user);
            $user->refresh();
        }

        $this->secretGenerated = true;
        $this->setupSecret = decrypt($user->two_factor_secret);
        $this->qrCodeSvg = $user->twoFactorQrCodeSvg();

        $this->form->fill();
    }

    public function getTitle(): string|Htmlable
    {
        return __('Two-Factor Authentication Required');
    }

    public function getHeading(): string|Htmlable
    {
        return __('Two-Factor Authentication Required');
    }

    public function getSubheading(): string|Htmlable|null
    {
        return __('Admin accounts require two-factor authentication. Scan the QR code with your authenticator app and enter the code to continue.');
    }

    public function form(Schema $schema): Schema
    {
        return $schema
            ->schema([
                TextInput::make('code')
                    ->label(__('Verification Code'))
                    ->placeholder('000000')
                    ->required()
                    ->autocomplete('one-time-code')
                    ->autofocus()
                    ->extraInputAttributes([
                        'inputmode' => 'numeric',
                        'pattern' => '[0-9]*',
                        'maxlength' => 6,
                    ]),
            ])
            ->statePath('data');
    }

    public function confirm(): void
    {
        $data = $this->form->getState();
        $user = Filament::auth()->user();

        if (! $user) {
            $this->redirect(Filament::getPanel('admin')->getLoginUrl());

            return;
        }

        $valid = app(TwoFactorAuthenticationProvider::class)->verify(
            decrypt($user->two_factor_secret),
            $data['code']
        );

        if (! $valid) {
            Notification::make()
                ->title(__('Invalid Code'))
                ->body(__('The verification code is invalid. Please try again.'))
                ->danger()
                ->send();

            $this->form->fill();

            return;
        }

        $user->forceFill([
            'two_factor_confirmed_at' => now(),
        ])->save();

        Notification::make()
            ->title(__('Two-Factor Authentication Enabled'))
            ->success()
            ->send();

        $this->redirect(Filament::getPanel('admin')->getUrl());
    }

    public function logout(): void
    {
        // If admin doesn't want to set up 2FA, they can log out
        Auth::guard('admin')->logout();
        session()->invalidate();
        session()->regenerateToken();

        $this->redirect(Filament::getPanel('admin')->getLoginUrl());
    }

    protected function hasFullWidthFormActions(): bool
    {
        return true;
    }
}
