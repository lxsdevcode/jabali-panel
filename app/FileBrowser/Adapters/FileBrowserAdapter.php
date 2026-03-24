<?php

declare(strict_types=1);

namespace App\FileBrowser\Adapters;

interface FileBrowserAdapter
{
    /** Core file operations. Always available. */
    public function files(): FileOperations;

    /** Archive extraction. Returns null if unsupported. */
    public function archiver(): ?Archiver;

    /** Permission management. Returns null if unsupported. */
    public function permissions(): ?PermissionManager;
}
