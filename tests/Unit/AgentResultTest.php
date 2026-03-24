<?php

declare(strict_types=1);

namespace Tests\Unit;

use App\Services\Agent\AgentResult;
use Tests\TestCase;

class AgentResultTest extends TestCase
{
    public function test_successful_result_from_response(): void
    {
        $result = AgentResult::fromResponse(['success' => true, 'data' => 'hello']);

        $this->assertTrue($result->success);
        $this->assertFalse($result->failed());
        $this->assertNull($result->error);
    }

    public function test_failed_result_from_response(): void
    {
        $result = AgentResult::fromResponse(['success' => false, 'error' => 'Connection refused']);

        $this->assertFalse($result->success);
        $this->assertTrue($result->failed());
        $this->assertSame('Connection refused', $result->error);
    }

    public function test_error_falls_back_to_message_key(): void
    {
        $result = AgentResult::fromResponse(['success' => false, 'message' => 'Timeout']);

        $this->assertSame('Timeout', $result->error);
    }

    public function test_or_fail_throws_on_failure(): void
    {
        $result = AgentResult::fromResponse(['success' => false, 'error' => 'Disk full']);

        $this->expectException(\App\Services\Agent\AgentException::class);
        $this->expectExceptionMessage('Disk full');

        $result->orFail();
    }

    public function test_or_fail_returns_self_on_success(): void
    {
        $result = AgentResult::fromResponse(['success' => true]);

        $this->assertSame($result, $result->orFail());
    }

    public function test_get_returns_data_by_key(): void
    {
        $result = AgentResult::fromResponse([
            'success' => true,
            'domains' => ['example.com', 'test.com'],
            'count' => 42,
        ]);

        $this->assertSame(['example.com', 'test.com'], $result->get('domains'));
        $this->assertSame(42, $result->get('count'));
        $this->assertNull($result->get('missing'));
        $this->assertSame('fallback', $result->get('missing', 'fallback'));
    }

    public function test_to_array_returns_raw_response(): void
    {
        $response = ['success' => true, 'data' => 'test'];
        $result = AgentResult::fromResponse($response);

        $this->assertSame($response, $result->toArray());
    }

    public function test_handle_returns_result_on_success(): void
    {
        $called = false;

        $result = AgentResult::handle(
            fn () => ['success' => true, 'value' => 123],
            'Done',
            'Failed',
            onSuccess: function () use (&$called) {
                $called = true;
            },
        );

        $this->assertInstanceOf(AgentResult::class, $result);
        $this->assertTrue($called);
    }

    public function test_handle_returns_null_on_failure(): void
    {
        $result = AgentResult::handle(
            fn () => ['success' => false, 'error' => 'Bad input'],
            'Done',
            'Failed',
        );

        $this->assertNull($result);
    }

    public function test_handle_returns_null_on_exception(): void
    {
        $result = AgentResult::handle(
            fn () => throw new \RuntimeException('Socket error'),
            'Done',
            'Failed',
        );

        $this->assertNull($result);
    }
}
