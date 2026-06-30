import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:tiktok_clone/features/bookmarks/presentation/providers/bookmark_provider.dart';

class BookmarksScreen extends ConsumerStatefulWidget {
  const BookmarksScreen({super.key});

  @override
  ConsumerState<BookmarksScreen> createState() => _BookmarksScreenState();
}

class _BookmarksScreenState extends ConsumerState<BookmarksScreen> {
  final _scrollController = ScrollController();

  @override
  void initState() {
    super.initState();
    _scrollController.addListener(_onScroll);
  }

  void _onScroll() {
    if (_scrollController.position.pixels >=
        _scrollController.position.maxScrollExtent - 300) {
      ref.read(bookmarkedVideosProvider.notifier).loadMore();
    }
  }

  @override
  void dispose() {
    _scrollController.dispose();
    super.dispose();
  }

  void _onLongPress(BuildContext context, String videoId) {
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: const Color(0xFF1A1A1A),
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
      ),
      builder: (_) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const SizedBox(height: 8),
            Container(
              width: 40,
              height: 4,
              decoration: BoxDecoration(
                color: Colors.white24,
                borderRadius: BorderRadius.circular(2),
              ),
            ),
            const SizedBox(height: 8),
            ListTile(
              leading: const Icon(Icons.bookmark_remove, color: Colors.redAccent),
              title: const Text(
                'Remove from bookmarks',
                style: TextStyle(color: Colors.white70),
              ),
              onTap: () {
                Navigator.pop(context);
                ref
                    .read(bookmarkedVideosProvider.notifier)
                    .removeVideo(videoId);
              },
            ),
            const SizedBox(height: 8),
          ],
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final asyncState = ref.watch(bookmarkedVideosProvider);

    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        backgroundColor: Colors.black,
        title: const Text(
          'Bookmarks',
          style: TextStyle(color: Colors.white, fontWeight: FontWeight.w600),
        ),
        iconTheme: const IconThemeData(color: Colors.white),
      ),
      body: asyncState.when(
        loading: () => const Center(
          child: CircularProgressIndicator(color: Color(0xFFFF0050)),
        ),
        error: (e, _) => Center(
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              const Icon(Icons.error_outline, color: Colors.white54, size: 48),
              const SizedBox(height: 12),
              Text(
                e.toString(),
                style: const TextStyle(color: Colors.white54),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 16),
              ElevatedButton(
                onPressed: () => ref.invalidate(bookmarkedVideosProvider),
                style: ElevatedButton.styleFrom(
                    backgroundColor: const Color(0xFFFF0050)),
                child: const Text('Retry'),
              ),
            ],
          ),
        ),
        data: (state) {
          if (state.videos.isEmpty) {
            return const Center(
              child: Column(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  Icon(Icons.bookmark_border, color: Colors.white24, size: 64),
                  SizedBox(height: 16),
                  Text(
                    'No bookmarks yet',
                    style: TextStyle(color: Colors.white54, fontSize: 16),
                  ),
                  SizedBox(height: 8),
                  Text(
                    'Videos you save will appear here',
                    style: TextStyle(color: Colors.white38, fontSize: 13),
                  ),
                ],
              ),
            );
          }

          return GridView.builder(
            controller: _scrollController,
            padding: const EdgeInsets.all(2),
            gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
              crossAxisCount: 3,
              crossAxisSpacing: 2,
              mainAxisSpacing: 2,
              childAspectRatio: 9 / 16,
            ),
            itemCount: state.videos.length + (state.isLoadingMore ? 3 : 0),
            itemBuilder: (context, index) {
              if (index >= state.videos.length) {
                return Container(color: const Color(0xFF1A1A1A));
              }

              final video = state.videos[index];
              final videoId = video['id'] as String? ?? '';
              final thumbnailUrl = video['thumbnail_url'] as String? ?? '';

              return GestureDetector(
                onTap: () => context.push('/video/$videoId'),
                onLongPress: () => _onLongPress(context, videoId),
                child: Stack(
                  fit: StackFit.expand,
                  children: [
                    // Thumbnail
                    thumbnailUrl.isNotEmpty
                        ? Image.network(
                            thumbnailUrl,
                            fit: BoxFit.cover,
                            errorBuilder: (_, __, ___) => Container(
                              color: const Color(0xFF2A2A2A),
                              child: const Icon(Icons.broken_image,
                                  color: Colors.white24),
                            ),
                          )
                        : Container(
                            color: const Color(0xFF2A2A2A),
                            child: const Icon(Icons.video_library,
                                color: Colors.white24),
                          ),
                    // Play count overlay
                    Positioned(
                      bottom: 4,
                      left: 4,
                      child: Row(
                        children: [
                          const Icon(Icons.play_arrow,
                              color: Colors.white, size: 14),
                          const SizedBox(width: 2),
                          Text(
                            _formatCount(
                                (video['play_count'] as num?)?.toInt() ?? 0),
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
        },
      ),
    );
  }

  String _formatCount(int count) {
    if (count >= 1000000) return '${(count / 1000000).toStringAsFixed(1)}M';
    if (count >= 1000) return '${(count / 1000).toStringAsFixed(1)}K';
    return '$count';
  }
}
