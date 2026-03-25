<div>
    <x-tab-loading-skeleton />
    <div wire:loading.remove wire:target="activeTab" class="-mt-4">
        {{ $this->table }}
    </div>
</div>
