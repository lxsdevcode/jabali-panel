<?php

declare(strict_types=1);

namespace App\Console\Commands\Cli;

use App\Console\Cli\JabaliCommand;
use Illuminate\Console\Command;
use Illuminate\Support\Str;
use Symfony\Component\Process\Process;

class LogsCommand extends JabaliCommand
{
    protected $signature = 'jabali:logs:share {--ttl=86400} {--raw} {--json} {--yes}';

    protected $description = 'Collect diagnostic logs and send to Jabali support';

    private const NTFY_URL = 'https://ntfy.jabali-panel.com/jabali-support';

    private const ENCLOSED_URL = 'https://enclosed.jabali-panel.com';

    protected function execute(\Symfony\Component\Console\Input\InputInterface $input, \Symfony\Component\Console\Output\OutputInterface $output): int
    {
        $this->setupFormatter();

        if ($this->option('raw')) {
            $this->line(implode("\n", $this->collectLogs()));

            return Command::SUCCESS;
        }

        $this->line('');
        $this->line('  Collects: services, configs, error logs.');
        $this->line('  No passwords or personal data. Sent encrypted to Jabali support.');
        $this->line('');
        if (! $this->confirmAction('Send diagnostic logs?')) {
            return Command::SUCCESS;
        }

        $userNote = '';
        if (! $this->option('yes')) {
            $userNote = (string) $this->ask('Describe the issue (optional, press Enter to skip)');
        }

        $this->formatter()->info('Collecting diagnostic logs...');

        $logs = $this->collectLogs();
        if ($userNote !== '') {
            array_splice($logs, 1, 0, ["User note: {$userNote}", '']);
        }
        $report = implode("\n", $logs);

        // Check if node is available
        $nodeCheck = Process::fromShellCommandline('which node 2>/dev/null');
        $nodeCheck->run();
        if ($nodeCheck->getExitCode() !== 0) {
            $this->formatter()->error('Node.js not found — required for encrypted paste');
            $this->formatter()->info('Use --raw to output logs to terminal instead');

            return Command::FAILURE;
        }

        $ttl = max(3600, min(604800, (int) $this->option('ttl')));
        $password = Str::random(16);
        $hostname = gethostname() ?: 'unknown';

        // Install enclosed CLI globally on first run
        $checkInstalled = Process::fromShellCommandline('npm list -g @enclosed/cli 2>/dev/null | grep -q enclosed');
        $checkInstalled->run();
        if ($checkInstalled->getExitCode() !== 0) {
            $this->formatter()->info('Installing enclosed CLI (first time only)...');
            $install = Process::fromShellCommandline('npm install -g @enclosed/cli 2>/dev/null');
            $install->setTimeout(60);
            $install->run();
        }

        $this->formatter()->info('Uploading encrypted logs...');

        $process = new Process([
            'enclosed', 'create',
            '--instance-url', self::ENCLOSED_URL,
            '--ttl', (string) $ttl,
            '--password', $password,
            '--stdin',
        ]);
        $process->setInput($report);
        $process->setTimeout(30);
        $process->run();

        $cliOutput = trim($process->getOutput().$process->getErrorOutput());

        if ($process->getExitCode() !== 0) {
            $this->formatter()->error('Upload failed: '.$cliOutput);
            $this->formatter()->info('Use --raw to output logs to terminal instead');

            return Command::FAILURE;
        }

        $url = $this->extractUrl($cliOutput);
        $linkUrl = $url ?: $cliOutput;
        $hours = intdiv($ttl, 3600);

        // Generate a ticket ID so the user can reference it in issues
        $ticketId = strtoupper(substr(md5($linkUrl), 0, 8));

        // Send link + password to Jabali support via ntfy
        $sent = $this->sendNtfy($linkUrl, $password, $hostname, $hours, $ticketId);

        if ($this->option('json')) {
            $this->formatter()->json([
                'ticket' => $ticketId,
                'ttl_seconds' => $ttl,
                'sent' => $sent,
            ]);

            return $sent ? Command::SUCCESS : Command::FAILURE;
        }

        if (! $sent) {
            $this->line('');
            $this->formatter()->error('Logs uploaded but could not notify support. Check network connectivity.');

            return Command::FAILURE;
        }

        $this->line('');
        $this->formatter()->success('Diagnostic logs sent to Jabali support.');
        $this->line('');
        $this->line("  Ticket ID: {$ticketId}");
        $this->line('');
        $this->formatter()->info("Share this ID in your GitHub issue so we can find your logs. Expires in {$hours} hour(s).");

        return Command::SUCCESS;
    }

    private function sendNtfy(string $url, string $password, string $hostname, int $hours, string $ticketId): bool
    {
        try {
            $process = new Process([
                'curl', '-s', '-f',
                '-H', "Title: [{$ticketId}] Diagnostic logs from {$hostname}",
                '-H', "Tags: stethoscope,{$hostname}",
                '-H', 'Priority: default',
                '-H', "Click: {$url}",
                '-d', "Ticket: {$ticketId}\nHost: {$hostname}\nLink: {$url}\nPassword: {$password}\nExpires: {$hours}h",
                self::NTFY_URL,
            ]);
            $process->setTimeout(10);
            $process->run();

            if ($process->getExitCode() !== 0) {
                $this->formatter()->error('Failed to notify support: '.trim($process->getErrorOutput()));

                return false;
            }

            return true;
        } catch (\Throwable $e) {
            $this->formatter()->error('Notification failed: '.$e->getMessage());

            return false;
        }
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
