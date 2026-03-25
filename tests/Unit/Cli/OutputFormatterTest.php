<?php

declare(strict_types=1);

namespace Tests\Unit\Cli;

use App\Console\Cli\OutputFormatter;
use PHPUnit\Framework\TestCase;

class OutputFormatterTest extends TestCase
{
    public function test_table_produces_columnar_output_with_headers(): void
    {
        $output = new \Symfony\Component\Console\Output\BufferedOutput();
        $formatter = new OutputFormatter($output, 'normal');

        $formatter->table(
            ['Name', 'Email'],
            [
                ['Alice', 'alice@example.com'],
                ['Bob', 'bob@example.com'],
            ]
        );

        $result = $output->fetch();

        $this->assertStringContainsString('Name', $result);
        $this->assertStringContainsString('Email', $result);
        $this->assertStringContainsString('Alice', $result);
        $this->assertStringContainsString('alice@example.com', $result);
        $this->assertStringContainsString('Bob', $result);
    }

    public function test_json_produces_valid_json(): void
    {
        $output = new \Symfony\Component\Console\Output\BufferedOutput();
        $formatter = new OutputFormatter($output, 'json');

        $formatter->json(['name' => 'Alice', 'email' => 'alice@example.com']);

        $result = $output->fetch();
        $decoded = json_decode($result, true);

        $this->assertNotNull($decoded);
        $this->assertSame('Alice', $decoded['name']);
        $this->assertSame('alice@example.com', $decoded['email']);
    }

    public function test_quiet_mode_suppresses_table_output(): void
    {
        $output = new \Symfony\Component\Console\Output\BufferedOutput();
        $formatter = new OutputFormatter($output, 'quiet');

        $formatter->table(
            ['Name'],
            [['Alice']],
        );

        $this->assertEmpty($output->fetch());
    }

    public function test_quiet_mode_suppresses_json_output(): void
    {
        $output = new \Symfony\Component\Console\Output\BufferedOutput();
        $formatter = new OutputFormatter($output, 'quiet');

        $formatter->json(['name' => 'Alice']);

        $this->assertEmpty($output->fetch());
    }

    public function test_quiet_mode_suppresses_info_output(): void
    {
        $output = new \Symfony\Component\Console\Output\BufferedOutput();
        $formatter = new OutputFormatter($output, 'quiet');

        $formatter->info('Hello');
        $formatter->success('Done');

        $this->assertEmpty($output->fetch());
    }

    public function test_error_outputs_even_in_quiet_mode(): void
    {
        $output = new \Symfony\Component\Console\Output\BufferedOutput();
        $errorOutput = new \Symfony\Component\Console\Output\BufferedOutput();
        $formatter = new OutputFormatter($output, 'quiet', $errorOutput);

        $formatter->error('Something broke');

        $this->assertEmpty($output->fetch());
        $this->assertStringContainsString('Something broke', $errorOutput->fetch());
    }

    public function test_error_outputs_in_normal_mode(): void
    {
        $output = new \Symfony\Component\Console\Output\BufferedOutput();
        $errorOutput = new \Symfony\Component\Console\Output\BufferedOutput();
        $formatter = new OutputFormatter($output, 'normal', $errorOutput);

        $formatter->error('Something broke');

        $this->assertStringContainsString('Something broke', $errorOutput->fetch());
    }
}
