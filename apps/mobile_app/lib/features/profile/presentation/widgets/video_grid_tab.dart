import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:cached_network_image/cached_network_image.dart';
import 'package:tiktok_clone/core/network/api_client.dart';
import 'package:tiktok_clone/features/home_feed/domain/entities/feed_item_entity.dart';
import 'package:tiktok_clone/features/profile/domain/usecases/get_user_videos_usecase.dart';
import 'package:tiktok_clone/features/profile/presentation/providers/profile_provider.dart';
import 'package:tiktok_clone/features/profile/presentation/widgets/profile_stats_row.dart'
    show formatCount;

const _kRed = Color(0xFFEE1D52);

// ─────────────────────────────────────────────────────────────────────────────
// Video thumbnail
// ─────────────────────────────────────────────────────────────────────────────

class VideoThumbnailCell extends ConsumerWidget {
  const VideoThumbnailCell({
    super.key,
    required this.item,
    this.isOwn = false,
    this.userId = '',
    this.tab = VideoTab.posted,
  });

  final FeedItemEntity item;
  final bool isOwn;
  final String userId;
  final VideoTab tab;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return GestureDetector(
      onTap: () => context.push('/video/${item.videoId}'),
      onLongPress: isOwn && tab == VideoTab.posted
          ? () => _showEditSheet(context, ref)
          : null,
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
              child: const Icon(Icons.play_circle_fill,
                  color: Colors.white24, size: 32),
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
          if (isOwn && tab == VideoTab.posted)
            Positioned(
              top: 4,
              right: 4,
              child: GestureDetector(
                onTap: () => _showEditSheet(context, ref),
                child: Container(
                  width: 26,
                  height: 26,
                  decoration: BoxDecoration(
                    color: Colors.black54,
                    borderRadius: BorderRadius.circular(13),
                  ),
                  child: const Icon(Icons.more_horiz,
                      color: Colors.white, size: 16),
                ),
              ),
            ),
        ],
      ),
    );
  }

  void _showEditSheet(BuildContext context, WidgetRef ref) {
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: const Color(0xFF1A1A1A),
      isScrollControlled: true,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (_) => _EditPostSheet(
        item: item,
        onSaved: () => ref.invalidate(
          userVideosProvider((userId: userId, tab: VideoTab.posted)),
        ),
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Edit post bottom sheet
// ─────────────────────────────────────────────────────────────────────────────

class _EditPostSheet extends StatefulWidget {
  const _EditPostSheet({required this.item, required this.onSaved});

  final FeedItemEntity item;
  final VoidCallback onSaved;

  @override
  State<_EditPostSheet> createState() => _EditPostSheetState();
}

class _EditPostSheetState extends State<_EditPostSheet> {
  late final TextEditingController _descCtrl;
  late bool _isPublic;
  late bool _allowComments;
  late bool _allowDuet;
  late bool _allowStitch;
  bool _saving = false;

  @override
  void initState() {
    super.initState();
    _descCtrl = TextEditingController(text: widget.item.description);
    _isPublic = true;
    _allowComments = true;
    _allowDuet = true;
    _allowStitch = true;
  }

  @override
  void dispose() {
    _descCtrl.dispose();
    super.dispose();
  }

  Future<void> _save() async {
    setState(() => _saving = true);
    try {
      final dio = ApiClient.instance.dio;
      await dio.put<Map<String, dynamic>>(
        '/videos/${widget.item.videoId}',
        data: {
          'description': _descCtrl.text.trim(),
          'is_public': _isPublic,
          'allow_comments': _allowComments,
          'allow_duet': _allowDuet,
          'allow_stitch': _allowStitch,
        },
      );
      widget.onSaved();
      if (mounted) Navigator.of(context).pop();
    } on DioException catch (e) {
      if (!mounted) return;
      setState(() => _saving = false);
      final msg = e.response?.data is Map
          ? (e.response!.data as Map)['error'] as String? ?? 'Save failed'
          : 'Save failed';
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(msg), backgroundColor: _kRed),
      );
    } catch (_) {
      if (!mounted) return;
      setState(() => _saving = false);
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Save failed'), backgroundColor: _kRed),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.fromLTRB(
          20, 16, 20, MediaQuery.of(context).viewInsets.bottom + 24),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Center(
            child: Container(
              width: 36,
              height: 4,
              decoration: BoxDecoration(
                color: Colors.white24,
                borderRadius: BorderRadius.circular(2),
              ),
            ),
          ),
          const SizedBox(height: 16),
          const Text(
            'Edit post',
            style: TextStyle(
                color: Colors.white,
                fontSize: 18,
                fontWeight: FontWeight.w700),
          ),
          const SizedBox(height: 16),
          TextField(
            controller: _descCtrl,
            maxLines: 4,
            maxLength: 300,
            style: const TextStyle(color: Colors.white, fontSize: 14),
            cursorColor: _kRed,
            decoration: InputDecoration(
              hintText: 'Add a caption...',
              hintStyle: const TextStyle(color: Colors.white38),
              counterStyle: const TextStyle(color: Colors.white38),
              filled: true,
              fillColor: Colors.white10,
              border: OutlineInputBorder(
                borderRadius: BorderRadius.circular(10),
                borderSide: BorderSide.none,
              ),
            ),
          ),
          const SizedBox(height: 8),
          _ToggleTile(
            icon: Icons.public_outlined,
            title: 'Public',
            value: _isPublic,
            onChanged: (v) => setState(() => _isPublic = v),
          ),
          _ToggleTile(
            icon: Icons.chat_bubble_outline,
            title: 'Allow comments',
            value: _allowComments,
            onChanged: (v) => setState(() => _allowComments = v),
          ),
          _ToggleTile(
            icon: Icons.people_outline,
            title: 'Allow Duet',
            value: _allowDuet,
            onChanged: (v) => setState(() => _allowDuet = v),
          ),
          _ToggleTile(
            icon: Icons.content_cut_outlined,
            title: 'Allow Stitch',
            value: _allowStitch,
            onChanged: (v) => setState(() => _allowStitch = v),
          ),
          const SizedBox(height: 16),
          SizedBox(
            width: double.infinity,
            height: 50,
            child: ElevatedButton(
              onPressed: _saving ? null : _save,
              style: ElevatedButton.styleFrom(
                backgroundColor: _kRed,
                disabledBackgroundColor: Colors.grey.shade800,
                shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(10)),
              ),
              child: _saving
                  ? const SizedBox(
                      width: 20,
                      height: 20,
                      child: CircularProgressIndicator(
                          strokeWidth: 2, color: Colors.white),
                    )
                  : const Text('Save',
                      style: TextStyle(
                          color: Colors.white,
                          fontSize: 16,
                          fontWeight: FontWeight.w700)),
            ),
          ),
        ],
      ),
    );
  }
}

class _ToggleTile extends StatelessWidget {
  const _ToggleTile({
    required this.icon,
    required this.title,
    required this.value,
    required this.onChanged,
  });

  final IconData icon;
  final String title;
  final bool value;
  final ValueChanged<bool> onChanged;

  @override
  Widget build(BuildContext context) {
    return SwitchListTile(
      contentPadding: EdgeInsets.zero,
      secondary: Icon(icon, color: Colors.white70, size: 20),
      title: Text(title, style: const TextStyle(color: Colors.white, fontSize: 14)),
      value: value,
      activeTrackColor: _kRed,
      onChanged: onChanged,
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
    this.isOwnProfile = false,
  });

  final String userId;
  final VideoTab tab;
  final bool isOwnProfile;

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
                item: gridState.items[index],
                isOwn: isOwnProfile,
                userId: userId,
                tab: tab,
              );
            },
          ),
        );
      },
    );
  }
}
