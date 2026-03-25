<div>
    <x-tab-loading-skeleton target="viewMode" />
    <div wire:loading.remove wire:target="viewMode" class="-mt-4">
        {{ $this->table }}
    </div>
</div>
