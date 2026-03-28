<?php

declare(strict_types=1);

namespace App\FileBrowser\Services;

use App\FileBrowser\Adapters\FileBrowserAdapter;
use App\FileBrowser\Support\PathSanitizer;
use App\FileBrowser\ValueObjects\FilePermission;
use App\Support\SafeError;
use Closure;
use Exception;
use Filament\Notifications\Notification;

final class FileOperationService
{
    private ?Closure $afterSuccess = null;

    public function __construct(
        private readonly FileBrowserAdapter $adapter,
    ) {}

    public function onSuccess(Closure $callback): self
    {
        $this->afterSuccess = $callback;

        return $this;
    }

    // ─── Single Operations ───────────────────────────────────

    public function mkdir(string $currentPath, string $folderName): void
    {
        $this->run(
            fn () => $this->adapter->files()->mkdir(
                $this->buildPath($currentPath, PathSanitizer::filename($folderName))
            ),
            __('Folder created'),
            __('Error creating folder'),
        );
    }

    public function createFile(string $currentPath, string $fileName, string $content = ''): void
    {
        $this->run(
            fn () => $this->adapter->files()->write(
                $this->buildPath($currentPath, PathSanitizer::filename($fileName)),
                $content,
            ),
            __('File created'),
            __('Error creating file'),
        );
    }

    public function saveFile(string $path, string $content): void
    {
        $this->run(
            fn () => $this->adapter->files()->write(PathSanitizer::clean($path), $content),
            __('File saved'),
            __('Error saving file'),
        );
    }

    public function rename(string $oldPath, string $newName): void
    {
        $safeName = PathSanitizer::filename($newName);
        $oldPath = PathSanitizer::clean($oldPath);
        $parentDir = dirname($oldPath);
        $newPath = ($parentDir !== '.' ? $parentDir.'/' : '').$safeName;

        $this->run(
            fn () => $this->adapter->files()->rename($oldPath, $newPath),
            __('Renamed successfully'),
            __('Error renaming'),
        );
    }

    public function delete(string $path): void
    {
        $this->run(
            fn () => $this->adapter->files()->delete(PathSanitizer::clean($path)),
            __('Deleted'),
            __('Error'),
        );
    }

    public function trashOrDelete(string $path, bool $trashEnabled): void
    {
        $path = PathSanitizer::clean($path);

        if ($trashEnabled) {
            $this->run(
                fn () => app(TrashManager::class)->trash($path),
                __('Moved to trash'),
                __('Error'),
            );
        } else {
            $this->run(
                fn () => $this->adapter->files()->delete($path),
                __('Deleted'),
                __('Error'),
            );
        }
    }

    public function extract(string $path): void
    {
        $this->run(
            fn () => $this->adapter->archiver()->extract(PathSanitizer::clean($path)),
            __('Archive extracted successfully'),
            __('Error extracting archive'),
        );
    }

    public function chmod(string $path, array $formData): void
    {
        $mode = ! empty($formData['mode'])
            ? $formData['mode']
            : FilePermission::fromToggles($formData)->toOctal();

        $this->run(
            fn () => $this->adapter->permissions()->chmod(PathSanitizer::clean($path), $mode),
            __('Permissions changed'),
            __('Error changing permissions'),
        );
    }

    public function upload(string $directory, string $filename, string $content): void
    {
        $safeName = PathSanitizer::filename($filename);

        $this->run(
            fn () => $this->adapter->files()->upload(
                PathSanitizer::clean($directory),
                $safeName,
                $content,
            ),
            __('Uploaded: :filename', ['filename' => $safeName]),
            __('Upload failed: :filename', ['filename' => $safeName]),
        );
    }

    public function move(string $source, string $destination): void
    {
        $this->run(
            fn () => $this->adapter->files()->move(
                PathSanitizer::clean($source),
                PathSanitizer::clean($destination),
            ),
            __('Moved successfully'),
            __('Error moving'),
        );
    }

    public function copy(string $source, string $destination): void
    {
        $this->run(
            fn () => $this->adapter->files()->copy(
                PathSanitizer::clean($source),
                PathSanitizer::clean($destination),
            ),
            __('Copied successfully'),
            __('Error copying'),
        );
    }

    // ─── Read-Only Operations ────────────────────────────────

    public function read(string $path): ?array
    {
        try {
            return $this->adapter->files()->read(PathSanitizer::clean($path));
        } catch (Exception $e) {
            Notification::make()
                ->title(__('Error reading file'))
                ->body(SafeError::message($e))
                ->danger()
                ->send();

            return null;
        }
    }

    public function info(string $path): ?array
    {
        try {
            return $this->adapter->files()->info(PathSanitizer::clean($path));
        } catch (Exception) {
            return null;
        }
    }

    // ─── Bulk Operations ─────────────────────────────────────

    public function bulkTrashOrDelete(iterable $records, bool $trashEnabled): void
    {
        $this->runBulk(
            $records,
            function (array $record) use ($trashEnabled) {
                $path = $record['path'];
                if ($trashEnabled) {
                    app(TrashManager::class)->trash($path);
                } else {
                    $this->adapter->files()->delete($path);
                }
            },
            $trashEnabled
                ? ':count item(s) moved to trash'
                : ':count item(s) deleted',
            ':count item(s) failed',
        );
    }

    public function bulkMove(iterable $records, string $destination): void
    {
        $destination = PathSanitizer::clean($destination);

        $this->runBulk(
            $records,
            fn (array $record) => $this->adapter->files()->move(
                $record['path'],
                $destination.'/'.basename($record['path']),
            ),
            ':count item(s) moved',
            ':count item(s) failed to move',
        );
    }

    public function bulkCopy(iterable $records, string $destination): void
    {
        $destination = PathSanitizer::clean($destination);

        $this->runBulk(
            $records,
            fn (array $record) => $this->adapter->files()->copy(
                $record['path'],
                $destination.'/'.basename($record['path']),
            ),
            ':count item(s) copied',
            ':count item(s) failed to copy',
        );
    }

    public function bulkUpload(array $files, string $directory): void
    {
        $directory = PathSanitizer::clean($directory);

        $this->runBulk(
            $files,
            function ($file) use ($directory) {
                $filename = PathSanitizer::filename($file->getClientOriginalName());
                $content = file_get_contents($file->getRealPath());
                $this->adapter->files()->upload($directory, $filename, $content);
            },
            ':count file(s) uploaded',
            ':count file(s) failed to upload',
        );
    }

    // ─── Internals ───────────────────────────────────────────

    private function run(Closure $operation, string $successMessage, string $errorMessage): void
    {
        try {
            $operation();

            Notification::make()->title($successMessage)->success()->send();

            if ($this->afterSuccess) {
                ($this->afterSuccess)();
            }
        } catch (Exception $e) {
            Notification::make()
                ->title($errorMessage)
                ->body(SafeError::message($e))
                ->danger()
                ->send();
        }
    }

    private function runBulk(
        iterable $items,
        Closure $operation,
        string $successMessage,
        string $failMessage,
    ): void {
        $succeeded = 0;
        $failed = 0;

        foreach ($items as $item) {
            try {
                $operation($item);
                $succeeded++;
            } catch (Exception $e) {
                $failed++;
            }
        }

        if ($succeeded > 0) {
            Notification::make()
                ->title(__($successMessage, ['count' => $succeeded]))
                ->success()
                ->send();
        }
        if ($failed > 0) {
            Notification::make()
                ->title(__($failMessage, ['count' => $failed]))
                ->danger()
                ->send();
        }

        if ($succeeded > 0 && $this->afterSuccess) {
            ($this->afterSuccess)();
        }
    }

    private function buildPath(string $currentPath, string $name): string
    {
        return empty($currentPath) ? $name : $currentPath.'/'.$name;
    }
}
