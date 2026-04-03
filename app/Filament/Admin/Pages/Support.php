<?php

declare(strict_types=1);

namespace App\Filament\Admin\Pages;

use App\Support\SafeError;
use BackedEnum;
use Exception;
use Filament\Actions\Action;
use Filament\Notifications\Notification;
use Filament\Pages\Page;
use Illuminate\Contracts\Support\Htmlable;
use Illuminate\Support\Facades\Artisan;

class Support extends Page
{
    protected static string|BackedEnum|null $navigationIcon = 'heroicon-o-question-mark-circle';

    protected static ?int $navigationSort = 24;

    protected static ?string $slug = 'support';

    protected string $view = 'filament.admin.pages.support';

    public static function getNavigationLabel(): string
    {
        return __('Support');
    }

    public function getTitle(): string|Htmlable
    {
        return __('Support');
    }

    protected function getHeaderActions(): array
    {
        return [
            Action::make('diagnostic_report')
                ->label(__('Diagnostic Report'))
                ->icon('heroicon-o-document-arrow-down')
                ->color('gray')
                ->modalHeading(__('Send Diagnostic Report'))
                ->modalDescription(__('Collects server logs and sends them encrypted to Jabali support.'))
                ->modalSubmitActionLabel(__('Send to Support'))
                ->requiresConfirmation()
                ->action(function (): void {
                    try {
                        Artisan::call('jabali:logs:share', ['--json' => true]);
                        $output = json_decode(trim(Artisan::output()), true);
                        $sent = $output['sent'] ?? false;

                        if (! $sent) {
                            Notification::make()
                                ->title(__('Failed to send'))
                                ->body(__('Could not upload logs. Run "jabali logs share --raw" on the CLI.'))
                                ->danger()
                                ->send();

                            return;
                        }

                        Notification::make()
                            ->title(__('Diagnostic report sent'))
                            ->body(__('Encrypted logs have been sent to Jabali support. They will review them shortly.'))
                            ->success()
                            ->send();
                    } catch (Exception $e) {
                        Notification::make()
                            ->title(__('Report generation failed'))
                            ->body(SafeError::message($e))
                            ->danger()
                            ->send();
                    }
                }),
        ];
    }
}
