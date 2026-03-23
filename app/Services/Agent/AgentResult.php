<?php

declare(strict_types=1);

namespace App\Services\Agent;

use App\Support\SafeError;
use Closure;
use Filament\Notifications\Notification;

final class AgentResult
{
    private function __construct(
        public readonly bool $success,
        public readonly array $data,
        public readonly ?string $error,
    ) {}

    public static function fromResponse(array $response): self
    {
        return new self(
            success: $response['success'] ?? false,
            data: $response,
            error: $response['error'] ?? $response['message'] ?? null,
        );
    }

    public function failed(): bool
    {
        return ! $this->success;
    }

    public function orFail(): self
    {
        if ($this->failed()) {
            throw new AgentException($this->error ?? __('Agent call failed'));
        }

        return $this;
    }

    public function get(string $key, mixed $default = null): mixed
    {
        return $this->data[$key] ?? $default;
    }

    public function toArray(): array
    {
        return $this->data;
    }

    /**
     * Call the agent, handle success/failure notifications, and run callbacks.
     *
     * - On success: runs $onSuccess callback, shows success notification.
     * - On failure: runs $onFailure callback or shows error notification with agent error message.
     * - On exception: shows safe error notification via SafeError.
     *
     * Returns AgentResult on success, null on failure/exception.
     */
    public static function handle(
        callable $agentCall,
        string $successTitle,
        string $errorTitle,
        ?Closure $onSuccess = null,
        ?Closure $onFailure = null,
    ): ?self {
        try {
            $response = $agentCall();

            $result = $response instanceof self
                ? $response
                : self::fromResponse($response);

            if ($result->failed()) {
                if ($onFailure) {
                    $onFailure($result);
                } else {
                    Notification::make()
                        ->title(__($errorTitle))
                        ->body($result->error ?? __('Unknown error'))
                        ->danger()
                        ->send();
                }

                return null;
            }

            if ($onSuccess) {
                $onSuccess($result);
            }

            Notification::make()
                ->title(__($successTitle))
                ->success()
                ->send();

            return $result;
        } catch (\Throwable $e) {
            Notification::make()
                ->title(__($errorTitle))
                ->body(SafeError::message($e))
                ->danger()
                ->send();

            return null;
        }
    }
}
