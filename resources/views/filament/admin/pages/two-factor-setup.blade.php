<x-filament-panels::page.simple>
    @if($this->secretGenerated && $this->qrCodeSvg)
        <div class="flex justify-center">
            {!! $this->qrCodeSvg !!}
        </div>

        <div class="text-center text-sm text-gray-500 dark:text-gray-400">
            {{ __('Or enter this code manually:') }}
            <code class="block mt-1 text-xs break-all">{{ $this->setupSecret }}</code>
        </div>
    @endif

    <form wire:submit="confirm" class="space-y-4">
        {{ $this->form }}

        <x-filament::button
            type="submit"
            class="w-full"
        >
            {{ __('Verify & Enable') }}
        </x-filament::button>
    </form>

    <div class="text-center mt-4">
        <x-filament::link
            tag="button"
            wire:click="logout"
            color="gray"
            size="sm"
        >
            {{ __('Log out instead') }}
        </x-filament::link>
    </div>
</x-filament-panels::page.simple>
