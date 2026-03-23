<?php

declare(strict_types=1);

namespace App\Services\Agent;

use Closure;

trait InteractsWithAgent
{
    protected function agent(): AgentClient
    {
        return app(AgentClient::class);
    }

    /**
     * Call agent action, check success, notify, and run callback.
     *
     * Returns AgentResult on success, null on failure/exception.
     */
    protected function agentCall(
        string $action,
        array $params,
        string $successTitle,
        string $errorTitle,
        ?Closure $onSuccess = null,
        ?Closure $onFailure = null,
    ): ?AgentResult {
        return AgentResult::handle(
            fn () => $this->agent()->send($action, $params),
            $successTitle,
            $errorTitle,
            $onSuccess,
            $onFailure,
        );
    }
}
