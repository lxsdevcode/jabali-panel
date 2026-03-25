@php
    $items = $this->getSecurityRecommendations();
@endphp

<div class="grid grid-cols-1 gap-8 md:grid-cols-2">
    @foreach ($items as $item)
        @php
            $ok = $item['ok'] ?? false;
            $icon = $ok ? 'heroicon-o-check-circle' : 'heroicon-o-exclamation-triangle';
            $colorClass = $ok ? 'text-success-500' : 'text-warning-500';
        @endphp
        <div class="flex items-start gap-3">
            <x-filament::icon :icon="$icon" class="{{ $colorClass }}" />
            <div>
                <h3 class="fi-section-header-heading">{{ $item['title'] }}</h3>
                <p class="fi-section-header-description">{{ $item['description'] }}</p>
            </div>
        </div>
    @endforeach
</div>
