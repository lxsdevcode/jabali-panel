<?php

// OPcache preloading for FrankenPHP
// Preloads frequently used Laravel framework files into shared memory

require __DIR__.'/../vendor/autoload.php';

// Preload core Laravel files
$files = new RecursiveIteratorIterator(
    new RecursiveDirectoryIterator(__DIR__.'/../vendor/laravel/framework/src/Illuminate/Support'),
    RecursiveIteratorIterator::LEAVES_ONLY
);

foreach ($files as $file) {
    if ($file->isFile() && $file->getExtension() === 'php') {
        opcache_compile_file($file->getRealPath());
    }
}
