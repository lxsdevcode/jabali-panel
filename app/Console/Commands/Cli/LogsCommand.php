<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;
use Symfony\Component\Process\Process;

class LogsCommand extends JabaliCommand
{
    protected $signature = 'jabali:logs:share {--ttl=86400} {--raw} {--json} {--yes}';

    protected $description = 'Collect diagnostic logs and share via encrypted paste';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        $this->formatter()->info('Collecting diagnostic logs...');

        $logs = $this->collectLogs();
        $report = implode("\n", $logs);

        if ($this->option('raw')) {
            $this->line($report);

            return Command::SUCCESS;
        }

        // Check if npx is available
        $npxCheck = Process::fromShellCommandline('which npx 2>/dev/null');
        $npxCheck->run();
        if ($npxCheck->getExitCode() !== 0) {
            $this->formatter()->error('npx not found — Node.js is required for encrypted sharing');
            $this->formatter()->info('Use --raw to output logs to terminal instead');

            return Command::FAILURE;
        }

        $ttl = max(3600, min(604800, (int) $this->option('ttl')));
        $instanceUrl = 'https://enclosed.jabali-panel.com';

        $this->formatter()->info('Uploading to encrypted paste...');

        $process = new Process([
            'npx', '--yes', '@enclosed/cli', 'create',
            '--instance-url', $instanceUrl,
            '--ttl', (string) $ttl,
            '--stdin',
        ]);
        $process->setInput($report);
        $process->setTimeout(30);
        $process->run();

        $cliOutput = trim($process->getOutput().$process->getErrorOutput());

        if ($process->getExitCode() !== 0) {
            $this->formatter()->error('Failed to upload: '.$cliOutput);
            $this->formatter()->info('Use --raw to output logs to terminal instead');

            return Command::FAILURE;
        }

        // Extract URL from CLI output
        $url = $this->extractUrl($cliOutput);

        if ($this->option('json')) {
            $this->formatter()->json([
                'url' => $url ?: $cliOutput,
                'ttl_seconds' => $ttl,
            ]);

            return Command::SUCCESS;
        }

        $this->line('');
        if ($url) {
            $this->formatter()->success('Diagnostic logs shared:');
            $this->line('');
            $this->line("  {$url}");
        } else {
            $this->line($cliOutput);
        }
        $this->line('');
        $hours = intdiv($ttl, 3600);
        $this->formatter()->info("Link expires in {$hours} hour(s).");

        return Command::SUCCESS;
    }

    private function collectLogs(): array
    {
        $logs = [];
        $logs[] = '=== JABALI DIAGNOSTIC LOGS ===';
        $logs[] = 'Generated: '.date('Y-m-d H:i:s T');
        $logs[] = 'Hostname: '.(gethostname() ?: 'unknown');
        $logs[] = '';

        $sections = [
            'System Info' => [
                'uname -a',
                'cat /etc/os-release 2>/dev/null | head -5',
                'uptime',
                'free -h',
                'df -h / /home',
            ],
            'Jabali Version' => [
                'cat '.escapeshellarg(base_path('VERSION')).' 2>/dev/null',
                'php -v | head -1',
            ],
            'Services' => [
                'systemctl is-active nginx mariadb stalwart-mail pdns redis-server jabali-agent jabali-queue jabali-panel bulwark 2>/dev/null',
                'systemctl --failed --no-pager 2>/dev/null | head -20',
            ],
            'SSH Config' => [
                'grep -iE "pubkey|authorized|allow|deny|match|forcecommand" /etc/ssh/sshd_config 2>/dev/null',
            ],
            'SSH Auth Log (last 30)' => [
                'journalctl -u ssh --since "1 hour ago" --no-pager 2>/dev/null | tail -30',
            ],
            'Shell Users' => [
                'getent group shellusers 2>/dev/null',
                'getent group sftpusers 2>/dev/null',
            ],
            'Home Directories' => [
                'ls -la /home/ 2>/dev/null',
            ],
            'Listening Ports' => [
                'ss -tlnp 2>/dev/null | head -25',
            ],
            'Nginx Error Log (last 20)' => [
                'tail -20 /var/log/nginx/error.log 2>/dev/null',
            ],
            'PHP-FPM Log (last 20)' => [
                'tail -20 /var/log/php*/php*-fpm.log 2>/dev/null',
            ],
            'Laravel Log (last 30)' => [
                'tail -30 '.escapeshellarg(base_path('storage/logs/laravel.log')).' 2>/dev/null',
            ],
            'Agent Log (last 20)' => [
                'tail -20 /var/log/jabali/agent.log 2>/dev/null',
            ],
            'Health Monitor (last 20)' => [
                'tail -20 /var/log/jabali/health-monitor.log 2>/dev/null',
            ],
            'Firewall' => [
                'ufw status 2>/dev/null | head -15',
            ],
            'Containers' => [
                'machinectl list --no-pager 2>/dev/null',
            ],
        ];

        foreach ($sections as $title => $commands) {
            $logs[] = "--- {$title} ---";
            foreach ($commands as $cmd) {
                $process = Process::fromShellCommandline($cmd);
                $process->setTimeout(10);
                $process->run();
                $out = trim($process->getOutput().$process->getErrorOutput());
                if ($out !== '') {
                    $logs[] = $out;
                }
            }
            $logs[] = '';
        }

        return $logs;
    }

    private function extractUrl(string $output): ?string
    {
        if (preg_match('/(https?:\/\/\S+)/', $output, $matches)) {
            return $matches[1];
        }

        return null;
    }
}
