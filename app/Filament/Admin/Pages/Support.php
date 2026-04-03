<?php

declare(strict_types=1);

namespace App\Filament\Admin\Pages;

use App\Support\SafeError;
use BackedEnum;
use Exception;
use Filament\Actions\Action;
use Filament\Forms\Components\Textarea;
use Filament\Infolists\Components\TextEntry;
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

    public string $ticketId = '';

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
                ->label(__('Send Diagnostic Report'))
                ->icon('heroicon-o-document-arrow-down')
                ->color('gray')
                ->modalHeading(__('Send Diagnostic Report'))
                ->modalDescription(__('Collects service status, configs, and error logs. No passwords or personal data. Sent encrypted to Jabali support.'))
                ->modalSubmitActionLabel(__('Send to Support'))
                ->form([
                    Textarea::make('note')
                        ->label(__('Describe the issue (optional)'))
                        ->placeholder(__('e.g. SSL not renewing, email delivery failing...'))
                        ->rows(3),
                ])
                ->action(function (array $data): void {
                    try {
                        $args = ['--json' => true, '--yes' => true];
                        Artisan::call('jabali:logs:share', $args);
                        $output = json_decode(trim(Artisan::output()), true);
                        $sent = $output['sent'] ?? false;
                        $this->ticketId = $output['ticket'] ?? '';

                        if (! $sent) {
                            Notification::make()
                                ->title(__('Failed to send'))
                                ->body(__('Could not reach support server. Run "jabali logs share" on the CLI.'))
                                ->danger()
                                ->send();

                            return;
                        }

                        $this->mountAction('showTicket');
                    } catch (Exception $e) {
                        Notification::make()
                            ->title(__('Report generation failed'))
                            ->body(SafeError::message($e))
                            ->danger()
                            ->send();
                    }
                }),

            Action::make('showTicket')
                ->hidden()
                ->modalHeading(__('Report Sent'))
                ->modalDescription(__('Your diagnostic report has been sent to Jabali support.'))
                ->modalWidth('md')
                ->modalSubmitAction(false)
                ->modalCancelActionLabel(__('Close'))
                ->infolist([
                    TextEntry::make('ticket')
                        ->label(__('Ticket ID'))
                        ->state(fn () => $this->ticketId)
                        ->copyable()
                        ->fontFamily('mono')
                        ->helperText(__('Share this ID in your support request so we can find your logs.')),
                ]),
        ];
    }
}
