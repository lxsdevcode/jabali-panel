<?php

declare(strict_types=1);

namespace App\FileBrowser\Support;

use RuntimeException;

final class PathSanitizer
{
    private string $rootPath;

    /** @var \Closure(string, string, bool): string */
    private \Closure $boundaryCheck;

    private function __construct(string $rootPath, \Closure $boundaryCheck)
    {
        $this->rootPath = $rootPath;
        $this->boundaryCheck = $boundaryCheck;
    }

    public static function local(string $rootPath): self
    {
        return new self($rootPath, self::realpathBoundaryCheck());
    }

    public static function remote(string $basePath): self
    {
        return new self($basePath, self::noBoundaryCheck());
    }

    public function resolve(string $path, bool $allowNew = false): string
    {
        $clean = self::clean($path);

        if ($clean === '') {
            return $this->rootPath;
        }

        $fullPath = $this->rootPath.'/'.$clean;

        return ($this->boundaryCheck)($this->rootPath, $fullPath, $allowNew);
    }

    public static function clean(string $path): string
    {
        $path = str_replace("\0", '', $path);
        $path = str_replace('\\', '/', $path);
        $path = trim($path, "/ \t\n\r");

        if ($path !== '' && $path[0] === '/') {
            throw new RuntimeException('Invalid path: absolute paths not allowed.');
        }

        if (str_starts_with($path, '~')) {
            throw new RuntimeException('Invalid path: home directory shortcuts not allowed.');
        }

        if (preg_match('/(?:^|\/)\.\.(\/|$)/', $path)) {
            throw new RuntimeException('Invalid path: directory traversal not allowed.');
        }

        $segments = explode('/', $path);
        $clean = [];
        foreach ($segments as $segment) {
            if ($segment === '' || $segment === '.') {
                continue;
            }
            if ($segment === '..') {
                throw new RuntimeException('Invalid path: directory traversal not allowed.');
            }
            $clean[] = $segment;
        }

        return implode('/', $clean);
    }

    public static function filename(string $filename): string
    {
        $filename = str_replace("\0", '', $filename);
        $filename = basename(str_replace('\\', '/', $filename));

        if ($filename === '' || preg_match('/^\.+$/', $filename)) {
            throw new RuntimeException('Invalid filename.');
        }

        return $filename;
    }

    private static function realpathBoundaryCheck(): \Closure
    {
        return static function (string $rootPath, string $fullPath, bool $allowNew): string {
            $realRoot = realpath($rootPath);
            if ($realRoot === false) {
                throw new RuntimeException('Root path is not accessible.');
            }

            if ($allowNew) {
                $parentDir = dirname($fullPath);
                $realParent = realpath($parentDir);

                if ($realParent === false) {
                    throw new RuntimeException('Parent directory does not exist.');
                }

                if ($realParent !== $realRoot && ! str_starts_with($realParent, $realRoot.'/')) {
                    throw new RuntimeException('Access denied: path is outside the root directory.');
                }

                return $realParent.'/'.basename($fullPath);
            }

            $realPath = realpath($fullPath);

            if ($realPath === false) {
                throw new RuntimeException('Path not found.');
            }

            if ($realPath !== $realRoot && ! str_starts_with($realPath, $realRoot.'/')) {
                throw new RuntimeException('Access denied: path is outside the root directory.');
            }

            return $realPath;
        };
    }

    private static function noBoundaryCheck(): \Closure
    {
        return static function (string $rootPath, string $fullPath, bool $allowNew): string {
            return $fullPath;
        };
    }
}
