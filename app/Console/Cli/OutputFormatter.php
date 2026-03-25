<?php

declare(strict_types=1);

namespace App\Console\Cli;

use Symfony\Component\Console\Helper\Table;
use Symfony\Component\Console\Output\OutputInterface;

class OutputFormatter
{
    public function __construct(
        private OutputInterface $output,
        private string $mode = 'normal',
        private ?OutputInterface $errorOutput = null,
    ) {
        $this->errorOutput = $errorOutput ?? $output;
    }

    public function table(array $headers, array $rows): void
    {
        if ($this->mode === 'quiet') {
            return;
        }

        $table = new Table($this->output);
        $table->setHeaders($headers);
        $table->setRows($rows);
        $table->render();
    }

    public function json(mixed $data): void
    {
        if ($this->mode === 'quiet') {
            return;
        }

        $this->output->writeln(json_encode($data, JSON_PRETTY_PRINT | JSON_UNESCAPED_UNICODE));
    }

    public function info(string $message): void
    {
        if ($this->mode === 'quiet') {
            return;
        }

        $this->output->writeln($message);
    }

    public function success(string $message): void
    {
        if ($this->mode === 'quiet') {
            return;
        }

        $this->output->writeln("<info>{$message}</info>");
    }

    public function error(string $message): void
    {
        $this->errorOutput->writeln("<error>{$message}</error>");
    }
}
