# Benchmarks

All benchmarks collected on:

```
goos:   linux
goarch: amd64
cpu:    AMD Ryzen 7 8845HS w/ Radeon 780M Graphics
go:     1.25
```

## go-wskit

| Benchmark | ns/op | B/op | allocs/op |
|-----------|------:|-----:|----------:|
| Hub_Broadcast_1Client | 376 | 387 | 5 |
| Hub_Broadcast_10Clients | 1,348 | 2,497 | 36 |
| Hub_Broadcast_100Clients | 14,973 | 33,767 | 503 |
| Hub_BroadcastEvent | 834 | 755 | 10 |
| Hub_BroadcastJSON | 884 | 771 | 10 |
| NewEvent | 47 | 0 | 0 |
| Event_Marshal | 515 | 288 | 6 |
| Client_Send | 31 | 0 | 0 |
| Hub_RegisterUnregister | 1,027 | 731 | 9 |

## go-cachekit

| Benchmark | ns/op | B/op | allocs/op |
|-----------|------:|-----:|----------:|
| BoundedCache_Set | 79 | 3 | 0 |
| BoundedCache_Get_Hit | 31 | 2 | 0 |
| BoundedCache_Get_Miss | 4.6 | 0 | 0 |
| BoundedCache_SetEviction | 67 | 0 | 0 |
| BoundedCache_Parallel_Get | 20 | 0 | 0 |
| BoundedCache_Parallel_SetGet | 16 | 0 | 0 |
| BoundedCache_Delete | 7.4 | 0 | 0 |
| CachedValue_Get_Hit | 121 | 0 | 0 |
| CachedValue_Get_Miss | 947 | 552 | 9 |
| CachedValue_Get_Parallel | 146 | 0 | 0 |
| CachedValue_Invalidate | 16 | 0 | 0 |
| GetOrLoad_CacheHit (Redis) | 121,659 | 640 | 13 |
| GetOrLoad_CacheMiss (Redis) | 415,790 | 1,630 | 34 |

## go-jwtkit

| Benchmark | ns/op | B/op | allocs/op |
|-----------|------:|-----:|----------:|
| HS256 GenerateTokenPair | 6,647 | 7,451 | 85 |
| HS256 ValidateAccessToken | 5,246 | 3,816 | 64 |
| HS256 ValidateRefreshToken | 5,614 | 3,824 | 64 |
| HS256 RefreshTokens | 13,675 | 11,356 | 149 |
| RS256 GenerateTokenPair | 1,558,933 | 9,587 | 79 |
| RS256 ValidateAccessToken | 29,127 | 5,024 | 68 |
| ES256 GenerateTokenPair | 56,873 | 19,711 | 206 |
| ES256 ValidateAccessToken | 60,361 | 4,672 | 80 |
| EdDSA GenerateTokenPair | 42,378 | 6,874 | 77 |
| EdDSA ValidateAccessToken | 45,959 | 3,296 | 57 |

## go-httpkit

| Benchmark | ns/op | B/op | allocs/op |
|-----------|------:|-----:|----------:|
| HandleError_HTTPError | 2,041 | 1,224 | 16 |
| HandleError_GenericError | 1,985 | 1,224 | 16 |
| DecodeAndValidate | 2,247 | 6,316 | 26 |
| DecodeAndValidateE | 1,972 | 6,156 | 23 |
| EscapeILIKE_Short | 112 | 416 | 1 |
| EscapeILIKE_WithSpecialChars | 110 | 416 | 1 |
| EscapeILIKE_Long | 501 | 416 | 1 |

## go-logkit

| Benchmark | ns/op | B/op | allocs/op |
|-----------|------:|-----:|----------:|
| InfoNoFields | 728 | 304 | 4 |
| InfoWithFields | 1,337 | 696 | 10 |
| SanitizeFields | 413 | 418 | 8 |

## Running locally

```bash
./run.sh
```

Requires all kit repositories cloned as siblings (`../go-wskit`, `../go-cachekit`, etc.).
