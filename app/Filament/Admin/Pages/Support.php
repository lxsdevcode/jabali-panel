<?php

declare(strict_types=1);

namespace App\Filament\Admin\Pages;

use App\Support\SafeError;
use BackedEnum;
use Exception;
use Filament\Actions\Action;
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

    public string $diagnosticUrl = '';

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
                ->modalHeading(__('Generating Report'))
                ->modalDescription(__('Collecting logs and uploading to encrypted paste...'))
                ->modalSubmitActionLabel(__('Generate & Share'))
                ->action(function (): void {
                    try {
                        Artisan::call('jabali:logs:share', ['--json' => true]);
                        $output = json_decode(trim(Artisan::output()), true);
                        $this->diagnosticUrl = $output['url'] ?? '';

                        if (empty($this->diagnosticUrl)) {
                            Notification::make()
                                ->title(__('Upload failed'))
                                ->body(__('Could not generate paste link. Run "jabali logs share --raw" on the CLI.'))
                                ->danger()
                                ->send();

                            return;
                        }

                        $this->mountAction('showDiagnosticLink');
                    } catch (Exception $e) {
                        Notification::make()
                            ->title(__('Report generation failed'))
                            ->body(SafeError::message($e))
                            ->danger()
                            ->send();
                    }
                }),

            Action::make('showDiagnosticLink')
                ->hidden()
                ->modalHeading(__('Diagnostic Report'))
                ->modalDescription(__('Share this link in a GitHub issue or with the Jabali team. It expires in 24 hours.'))
                ->modalWidth('md')
                ->modalSubmitAction(false)
                ->modalCancelActionLabel(__('Close'))
                ->infolist([
                    TextEntry::make('url')
                        ->label(__('Report URL'))
                        ->state(fn () => $this->diagnosticUrl)
                        ->copyable()
                        ->fontFamily('mono'),
                ]),
        ];
    }
}
