<?php

declare(strict_types=1);

namespace App\FileBrowser;

use App\FileBrowser\Pages\FileBrowser;
use Filament\Contracts\Plugin;
use Filament\Panel;

class FileBrowserPlugin implements Plugin
{
    protected bool $hasTrash = true;

    protected bool $hasExtract = true;

    protected bool $hasUpload = true;

    protected bool $hasEdit = true;

    protected bool $hasPermissions = true;

    protected ?string $navigationIcon = null;

    protected ?string $navigationLabel = null;

    protected ?int $navigationSort = null;

    protected ?string $navigationGroup = null;

    protected ?string $slug = null;

    public static function make(): static
    {
        return app(static::class);
    }

    public static function get(): static
    {
        return filament(static::class);
    }

    public function getId(): string
    {
        return 'file-browser';
    }

    public function register(Panel $panel): void
    {
        $panel->pages([
            FileBrowser::class,
        ]);
    }

    public function boot(Panel $panel): void
    {
        //
    }

    // Fluent config methods

    public function trash(bool $condition = true): static
    {
        $this->hasTrash = $condition;

        return $this;
    }

    public function extract(bool $condition = true): static
    {
        $this->hasExtract = $condition;

        return $this;
    }

    public function upload(bool $condition = true): static
    {
        $this->hasUpload = $condition;

        return $this;
    }

    public function edit(bool $condition = true): static
    {
        $this->hasEdit = $condition;

        return $this;
    }

    public function permissions(bool $condition = true): static
    {
        $this->hasPermissions = $condition;

        return $this;
    }

    public function navigationIcon(string $icon): static
    {
        $this->navigationIcon = $icon;

        return $this;
    }

    public function navigationLabel(string $label): static
    {
        $this->navigationLabel = $label;

        return $this;
    }

    public function navigationSort(int $sort): static
    {
        $this->navigationSort = $sort;

        return $this;
    }

    public function navigationGroup(string $group): static
    {
        $this->navigationGroup = $group;

        return $this;
    }

    public function slug(string $slug): static
    {
        $this->slug = $slug;

        return $this;
    }

    // Getters

    public function hasTrash(): bool
    {
        return $this->hasTrash;
    }

    public function hasExtract(): bool
    {
        return $this->hasExtract;
    }

    public function hasUpload(): bool
    {
        return $this->hasUpload;
    }

    public function hasEdit(): bool
    {
        return $this->hasEdit;
    }

    public function hasPermissions(): bool
    {
        return $this->hasPermissions;
    }

    public function getNavigationIcon(): ?string
    {
        return $this->navigationIcon;
    }

    public function getNavigationLabel(): ?string
    {
        return $this->navigationLabel;
    }

    public function getNavigationSort(): ?int
    {
        return $this->navigationSort;
    }

    public function getNavigationGroup(): ?string
    {
        return $this->navigationGroup;
    }

    public function getSlug(): ?string
    {
        return $this->slug;
    }
}
