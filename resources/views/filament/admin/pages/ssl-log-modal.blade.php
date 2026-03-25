<div class="overflow-hidden rounded-lg border border-gray-200 dark:border-white/10">
    <div class="max-h-[400px] overflow-auto bg-gray-950 font-mono text-sm leading-relaxed">
        <table class="w-full border-collapse">
            <tbody>
                @foreach(explode("\n", $log) as $index => $line)
                <tr class="hover:bg-white/5">
                    <td class="w-1 whitespace-nowrap bg-white/5 px-3 text-right text-gray-500 select-none align-top border-r border-white/10">{{ $index + 1 }}</td>
                    <td class="px-3 text-gray-200 whitespace-pre-wrap break-all">{{ $line }}</td>
                </tr>
                @endforeach
            </tbody>
        </table>
    </div>
</div>
