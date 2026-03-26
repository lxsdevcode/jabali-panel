<div class="flex flex-col items-center justify-center py-12 text-center">
    <x-filament::icon icon="heroicon-o-server-stack" class="mb-4 h-12 w-12 text-gray-400 dark:text-gray-500" />
    <h3 class="text-base font-semibold text-gray-950 dark:text-white">
        {{ __('PostgreSQL is not installed') }}
    </h3>
    <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
        {{ __('Contact your server administrator to install PostgreSQL.') }}
    </p>
</div>
