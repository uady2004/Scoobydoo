import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:cached_network_image/cached_network_image.dart';
import 'package:tiktok_clone/features/home_feed/domain/entities/feed_item_entity.dart';
import 'package:tiktok_clone/features/profile/domain/usecases/get_user_videos_usecase.dart';
import 'package:tiktok_clone/features/profile/presentation/providers/profile_provider.dart';
import 'package:tiktok_clone/features/profile/presentation/widgets/profile_stats_row.dart'
    show formatCount;

// ─────────────────────────────────────────────────────────────────────────────
// Video thumbnail
// ─────────────────────────────────────────────────────────────────────────────

class VideoThumbnailCell extends StatelessWidget {
  const VideoThumbnailCell({super.key, required this.item});

  final FeedItemEntity item;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: () => context.push('/video/${item.videoId}'),
      child: Stack(
        fit: StackFit.expand,
        children: [
          CachedNetworkImage(
            imageUrl: item.thumbnailUrl,
            fit: BoxFit.cover,
            placeholder: (_, __) =>
                Container(color: Colors.grey.shade900),
            errorWidget: (_, __, ___) => Container(
              color: Colors.grey.shade900,
              child: const Icon(Icons.broken_image_outlined,
                  color: Colors.white38),
            ),
          ),
          Positioned(
            bottom: 4,
            left: 4,
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                const Icon(Icons.play_arrow_rounded,
                    color: Colors.white, size: 14),
                const SizedBox(width: 2),
                Text(
                  formatCount(item.viewCount),
                  style: const TextStyle(
                    color: Colors.white,
                    fontSize: 11,
                    fontWeight: FontWeight.w600,
                    shadows: [Shadow(blurRadius: 4)],
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Empty state
// ─────────────────────────────────────────────────────────────────────────────

class VideoGridEmptyState extends StatelessWidget {
  const VideoGridEmptyState({super.key, required this.tab});

  final VideoTab tab;

  @override
  Widget build(BuildContext context) {
    final (icon, message) = switch (tab) {
      VideoTab.posted => (
          Icons.videocam_outlined,
          'No videos yet',
        ),
      VideoTab.liked => (
          Icons.favorite_border_rounded,
          'Liked videos are private',
        ),
      VideoTab.bookmarked => (
          Icons.bookmark_border_rounded,
          'No bookmarks yet',
        ),
    };

    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 52, color: Colors.white24),
          const SizedBox(height: 12),
          Text(message,
              style: const TextStyle(
                  color: Colors.white54, fontSize: 14)),
        ],
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Video grid tab — shared by ProfileScreen and CreatorProfileScreen
// ─────────────────────────────────────────────────────────────────────────────

class VideoGridTab extends ConsumerWidget {
  const VideoGridTab({
    super.key,
    required this.userId,
    required this.tab,
  });

  final String userId;
  final VideoTab tab;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final arg = (userId: userId, tab: tab);
    final asyncState = ref.watch(userVideosProvider(arg));

    return asyncState.when(
      loading: () =>
          const Center(child: CircularProgressIndicator()),
      error: (e, _) => Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(e.toString(),
                textAlign: TextAlign.center,
                style: const TextStyle(color: Colors.white54)),
            const SizedBox(height: 12),
            TextButton(
              onPressed: () =>
                  ref.read(userVideosProvider(arg).notifier).refresh(),
              child: const Text('Retry'),
            ),
          ],
        ),
      ),
      data: (gridState) {
        if (gridState.items.isEmpty && !gridState.isLoading) {
          return VideoGridEmptyState(tab: tab);
        }
        return NotificationListener<ScrollNotification>(
          onNotification: (n) {
            if (n is ScrollEndNotification &&
                n.metrics.extentAfter < 200) {
              ref.read(userVideosProvider(arg).notifier).loadMore();
            }
            return false;
          },
          child: GridView.builder(
            padding: EdgeInsets.zero,
            gridDelegate:
                const SliverGridDelegateWithFixedCrossAxisCount(
              crossAxisCount: 3,
              mainAxisSpacing: 1.5,
              crossAxisSpacing: 1.5,
              childAspectRatio: 9 / 16,
            ),
            itemCount: gridState.items.length +
                (gridState.isLoadingMore ? 1 : 0),
            itemBuilder: (context, index) {
              if (index >= gridState.items.length) {
                return const Center(
                    child: CircularProgressIndicator());
              }
              return VideoThumbnailCell(
                  item: gridState.items[index]);
            },
          ),
        );
      },
    );
  }
}
