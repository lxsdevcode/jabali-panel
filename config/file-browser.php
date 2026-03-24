<?php

return [
    // Default adapter: 'local', 'sftp', or 'ssh-rsync'
    'adapter' => env('FILE_BROWSER_ADAPTER', 'local'),

    // Max upload size in MB
    'max_upload_size_mb' => env('FILE_BROWSER_MAX_UPLOAD_MB', 100),

    // Warning banner text (set to null to hide)
    'warning_message' => 'Deleting or modifying system files can break your website.',

    // Warning folders to highlight
    'warning_folders' => ['conf', 'logs'],

    // How to resolve the username from the authenticated user
    // Supports: a string attribute name, or a callable class
    'user_resolver' => 'name',

    // Root path for local adapter
    'local' => [
        'root' => env('FILE_BROWSER_LOCAL_ROOT', ''),
    ],

    // SFTP connection settings
    'sftp' => [
        'host' => env('FILE_BROWSER_SFTP_HOST', ''),
        'port' => env('FILE_BROWSER_SFTP_PORT', 22),
        'username' => env('FILE_BROWSER_SFTP_USERNAME', ''),
        'password' => env('FILE_BROWSER_SFTP_PASSWORD'),
        'private_key' => env('FILE_BROWSER_SFTP_PRIVATE_KEY'),
        'passphrase' => env('FILE_BROWSER_SFTP_PASSPHRASE'),
        'root' => env('FILE_BROWSER_SFTP_ROOT', '/'),
        'timeout' => 30,
    ],

    // SSH/rsync settings
    'ssh_rsync' => [
        'host' => env('FILE_BROWSER_SSH_HOST', ''),
        'port' => env('FILE_BROWSER_SSH_PORT', 22),
        'username' => env('FILE_BROWSER_SSH_USERNAME', ''),
        'private_key' => env('FILE_BROWSER_SSH_PRIVATE_KEY'),
        'remote_path' => env('FILE_BROWSER_SSH_REMOTE_PATH', '/'),
        'rsync_options' => '-avz --progress',
        'timeout' => 60,
    ],

    // Navigation settings
    'navigation' => [
        'icon' => 'heroicon-o-folder',
        'label' => 'File Manager',
        'slug' => 'file-browser',
        'sort' => 6,
        'group' => null,
    ],

    // Features toggles
    'features' => [
        'trash' => true,
        'extract' => true,
        'upload' => true,
        'edit' => true,
        'permissions' => true,
        'drag_drop_upload' => true,
    ],
];
