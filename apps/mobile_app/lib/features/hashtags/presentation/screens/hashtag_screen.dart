import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:share_plus/share_plus.dart';

// ---------------------------------------------------------------------------
// Data model & provider
// ---------------------------------------------------------------------------

class HashtagScreenData {
  final String tag;
  final int videoCount;
  final bool isTrending;
  final List<Map<String, dynamic>> videos;
  final bool isLoading;

  const HashtagScreenData({
    required this.tag,
    this.videoCount = 0,
    this.isTrending = false,
    this.videos = const [],
    this.isLoading = true,
  });
}

final _hashtagProvider =
    FutureProvider.family<HashtagScreenData, String>((ref, tag) async {
  // Placeholder — replace with real repository call
  await Future<void>.delayed(const Duration(milliseconds: 600));
  return HashtagScreenData(
    tag: tag,
    videoCount: 12500000,
    isTrending: true,
    videos: List.generate(
      24,
      (i) => {
        'id': 'v$i',
        'thumbnail_url': '',
        'view_count': (i + 1) * 37400 + i * 1200,
      },
    ),
    isLoading: false,
  );
});

// ---------------------------------------------------------------------------
// Screen
// ---------------------------------------------------------------------------

class HashtagScreen extends ConsumerWidget {
  final String tag;

  const HashtagScreen({super.key, required this.tag});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final asyncData = ref.watch(_hashtagProvider(tag));

    return Scaffold(
      backgroundColor: Colors.black,
      body: asyncData.when(
        loading: () => const Center(
          child: CircularProgressIndicator(color: Color(0xFFFF0050)),
        ),
        error: (e, _) => Center(
          child: Text(e.toString(),
              style: const TextStyle(color: Colors.white54)),
        ),
        data: (data) => NestedScrollView(
          headerSliverBuilder: (context, innerBoxIsScrolled) => [
            _HashtagSliverAppBar(data: data),
          ],
          body: _VideoGrid(videos: data.videos),
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Sliver app bar with gradient header
// ---------------------------------------------------------------------------

class _HashtagSliverAppBar extends StatelessWidget {
  final HashtagScreenData data;

  const _HashtagSliverAppBar({required this.data});

  @override
  Widget build(BuildContext context) {
    return SliverAppBar(
      expandedHeight: 160,
      pinned: true,
      backgroundColor: Colors.black,
      iconTheme: const IconThemeData(color: Colors.white),
      actions: [
        IconButton(
          icon: const Icon(Icons.share_outlined, color: Colors.white),
          onPressed: () => SharePlus.instance.share(
            ShareParams(
              text:
                  'Check out #${data.tag} on TikTok Clone!\nhttps://tiktok-clone.dev/tag/${data.tag}',
            ),
          ),
        ),
      ],
      flexibleSpace: FlexibleSpaceBar(
        collapseMode: CollapseMode.pin,
        background: Stack(
          fit: StackFit.expand,
          children: [
            // Gradient background: red → purple → transparent (toward bottom)
            Container(
              decoration: const BoxDecoration(
                gradient: LinearGradient(
                  begin: Alignment.topLeft,
                  end: Alignment.bottomRight,
                  colors: [
                    Color(0xFFEE1D52),
                    Color(0xFF7928CA),
                    Color(0xFF00F2EA),
                  ],
                  stops: [0.0, 0.55, 1.0],
                ),
              ),
            ),
            // Scrim for readability
            Container(
              decoration: const BoxDecoration(
                gradient: LinearGradient(
                  begin: Alignment.topCenter,
                  end: Alignment.bottomCenter,
                  colors: [
                    Color(0x00000000),
                    Color(0xCC000000),
                  ],
                ),
              ),
            ),
            // Content pinned to bottom
            Positioned(
              bottom: 16,
              left: 16,
              right: 16,
              child: _HashtagHeader(data: data),
            ),
          ],
        ),
      ),
    );
  }
}

class _HashtagHeader extends StatelessWidget {
  final HashtagScreenData data;

  const _HashtagHeader({required this.data});

  @override
  Widget build(BuildContext context) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.center,
      children: [
        // Hash icon box
        Container(
          width: 52,
          height: 52,
          decoration: BoxDecoration(
            color: Colors.white.withValues(alpha: 0.15),
            borderRadius: BorderRadius.circular(12),
            border: Border.all(
              color: Colors.white.withValues(alpha: 0.25),
              width: 1,
            ),
          ),
          alignment: Alignment.center,
          child: const Text(
            '#',
            style: TextStyle(
              color: Colors.white,
              fontSize: 26,
              fontWeight: FontWeight.w800,
            ),
          ),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(
                '#${data.tag}',
                style: const TextStyle(
                  color: Colors.white,
                  fontSize: 22,
                  fontWeight: FontWeight.w800,
                  letterSpacing: -0.3,
                ),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
              const SizedBox(height: 4),
              Row(
                children: [
                  Text(
                    '${_fmt(data.videoCount)} Videos',
                    style: const TextStyle(
                      color: Colors.white70,
                      fontSize: 13,
                      fontWeight: FontWeight.w500,
                    ),
                  ),
                  if (data.isTrending) ...[
                    const SizedBox(width: 8),
                    Container(
                      padding: const EdgeInsets.symmetric(
                          horizontal: 8, vertical: 3),
                      decoration: BoxDecoration(
                        color: const Color(0xFFFF0050),
                        borderRadius: BorderRadius.circular(12),
                      ),
                      child: const Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Icon(Icons.trending_up,
                              size: 11, color: Colors.white),
                          SizedBox(width: 3),
                          Text(
                            'Trending',
                            style: TextStyle(
                              color: Colors.white,
                              fontSize: 11,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                        ],
                      ),
                    ),
                  ],
                ],
              ),
            ],
          ),
        ),
      ],
    );
  }

  String _fmt(int n) {
    if (n >= 1000000000) return '${(n / 1000000000).toStringAsFixed(1)}B';
    if (n >= 1000000) return '${(n / 1000000).toStringAsFixed(1)}M';
    if (n >= 1000) return '${(n / 1000).toStringAsFixed(1)}K';
    return '$n';
  }
}

// ---------------------------------------------------------------------------
// Video grid
// ---------------------------------------------------------------------------

class _VideoGrid extends StatelessWidget {
  final List<Map<String, dynamic>> videos;

  const _VideoGrid({required this.videos});

  @override
  Widget build(BuildContext context) {
    return GridView.builder(
      padding: const EdgeInsets.all(1),
      gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
        crossAxisCount: 3,
        crossAxisSpacing: 1.5,
        mainAxisSpacing: 1.5,
        childAspectRatio: 9 / 16,
      ),
      itemCount: videos.length,
      itemBuilder: (context, index) {
        final video = videos[index];
        final thumbnailUrl = video['thumbnail_url'] as String? ?? '';
        final videoId = video['id'] as String? ?? '';
        final viewCount = (video['view_count'] as num?)?.toInt() ?? 0;

        return GestureDetector(
          onTap: () => context.push('/video/$videoId'),
          child: Stack(
            fit: StackFit.expand,
            children: [
              thumbnailUrl.isNotEmpty
                  ? Image.network(thumbnailUrl,
                      fit: BoxFit.cover,
                      errorBuilder: (_, __, ___) =>
                          Container(color: const Color(0xFF1E1E1E)))
                  : Container(
                      color: Color(
                          (0xFF1A1A1A + (index * 0x050505)) & 0xFFFFFFFF),
                      child: const Icon(Icons.play_circle_outline,
                          color: Colors.white24, size: 28),
                    ),
              // View count bottom-left
              Positioned(
                bottom: 5,
                left: 5,
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    const Icon(Icons.play_arrow,
                        size: 13, color: Colors.white),
                    const SizedBox(width: 2),
                    Text(
                      _fmt(viewCount),
                      style: const TextStyle(
                        color: Colors.white,
                        fontSize: 11,
                        fontWeight: FontWeight.w600,
                        shadows: [
                          Shadow(blurRadius: 4, color: Colors.black54),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ),
        );
      },
    );
  }

  String _fmt(int n) {
    if (n >= 1000000) return '${(n / 1000000).toStringAsFixed(1)}M';
    if (n >= 1000) return '${(n / 1000).toStringAsFixed(1)}K';
    return '$n';
  }
}
