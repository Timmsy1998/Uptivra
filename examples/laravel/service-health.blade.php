{{-- Optional Blade partial. Set services.uptivra.origin in config/services.php. --}}
@php
    $labels = [
        'healthy' => 'All services operational',
        'degraded' => 'Some services degraded',
        'down' => 'Service interruption',
        'unknown' => 'Current health unavailable',
        'maintenance' => 'Scheduled maintenance',
        'paused' => 'Monitoring paused',
    ];
    $origin = rtrim(config('services.uptivra.origin', ''), '/');
    $state = 'unknown';
    if ($origin !== '') {
        try {
            $response = \Illuminate\Support\Facades\Http::timeout(5)
                ->connectTimeout(3)->withoutRedirecting()
                ->get($origin . '/api/v1/public/status');
            $candidate = $response->successful() ? $response->json('status') : null;
            if (is_string($candidate) && array_key_exists($candidate, $labels)) {
                $state = $candidate;
            }
        } catch (\Illuminate\Http\Client\ConnectionException $exception) {
            $state = 'unknown';
        }
    }
@endphp
@if ($origin !== '')
    <a href="{{ $origin }}/status">{{ $labels[$state] }}</a>
@else
    <span role="status">{{ $labels[$state] }}</span>
@endif
