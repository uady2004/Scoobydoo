import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../home_feed/domain/entities/feed_item_entity.dart';
import '../../../home_feed/presentation/providers/feed_provider.dart';
import '../widgets/tiktok_video_player.dart';

class VideoPlayerScreen extends ConsumerStatefulWidget {
  const VideoPlayerScreen({super.key, required this.videoId});

  final String videoId;

  @override
  ConsumerState<VideoPlayerScreen> createState() =>
      _VideoPlayerScreenState();
}

class _VideoPlayerScreenState extends ConsumerState<VideoPlayerScreen> {
  @override
  void initState() {
    super.initState();
    SystemChrome.setEnabledSystemUIMode(SystemUiMode.immersiveSticky);
  }

  @override
  void dispose() {
    SystemChrome.setEnabledSystemUIMode(
      SystemUiMode.manual,
      overlays: SystemUiOverlay.values,
    );
    super.dispose();
  }

  FeedItemEntity? _findCached() {
    final forYou = ref.read(forYouFeedProvider).valueOrNull;
    final found =
        forYou?.items.where((i) => i.videoId == widget.videoId).firstOrNull;
    if (found != null) return found;

    final following = ref.read(followingFeedProvider).valueOrNull;
    return following?.items
        .where((i) => i.videoId == widget.videoId)
        .firstOrNull;
  }

  @override
  Widget build(BuildContext context) {
    final item = _findCached();

    return Scaffold(
      backgroundColor: Colors.black,
      body: item != null
          ? _VideoBody(item: item)
          : _NotFoundBody(videoId: widget.videoId),
    );
  }
}

// ---------------------------------------------------------------------------
// Video body
// ---------------------------------------------------------------------------

class _VideoBody extends StatelessWidget {
  const _VideoBody({required this.item});

  final FeedItemEntity item;

  @override
  Widget build(BuildContext context) {
    return Stack(
      fit: StackFit.expand,
      children: [
        TikTokVideoPlayer(
          key: ValueKey(item.videoId),
          item: item,
          isActive: true,
          feedType: 'forYou',
          // ← FIXED: navigate to THIS video's creator profile
          onUsernameTap: () =>
              context.push('/profile/${item.creatorUsername}'),
          onCommentTap: () =>
              context.push('/comments/${item.videoId}'),
        ),
        Positioned(
          top: MediaQuery.of(context).padding.top + 8,
          left: 8,
          child: const _BackButton(),
        ),
      ],
    );
  }
}

// ---------------------------------------------------------------------------
// Not-found body
// ---------------------------------------------------------------------------

class _NotFoundBody extends StatelessWidget {
  const _NotFoundBody({required this.videoId});

  final String videoId;

  @override
  Widget build(BuildContext context) {
    return Stack(
      children: [
        Center(
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 32),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                const Icon(
                  Icons.videocam_off_outlined,
                  color: Colors.white38,
                  size: 64,
                ),
                const SizedBox(height: 16),
                const Text(
                  'Video unavailable',
                  style: TextStyle(
                    color: Colors.white,
                    fontSize: 18,
                    fontWeight: FontWeight.bold,
                  ),
                ),
                const SizedBox(height: 8),
                const Text(
                  'This video could not be loaded. It may have been removed or the link may have expired.',
                  style: TextStyle(color: Colors.white54, fontSize: 14),
                  textAlign: TextAlign.center,
                ),
                const SizedBox(height: 28),
                ElevatedButton(
                  onPressed: () => context.pop(),
                  style: ElevatedButton.styleFrom(
                    backgroundColor: const Color(0xFFFF0050),
                    foregroundColor: Colors.white,
                    padding: const EdgeInsets.symmetric(
                        horizontal: 28, vertical: 12),
                    shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(24)),
                  ),
                  child: const Text('Go back'),
                ),
              ],
            ),
          ),
        ),
        Positioned(
          top: MediaQuery.of(context).padding.top + 8,
          left: 8,
          child: const _BackButton(),
        ),
      ],
    );
  }
}

// ---------------------------------------------------------------------------
// Back button
// ---------------------------------------------------------------------------

class _BackButton extends StatelessWidget {
  const _BackButton();

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: () => context.pop(),
      child: Container(
        width: 36,
        height: 36,
        decoration: const BoxDecoration(
          color: Colors.black38,
          shape: BoxShape.circle,
        ),
        child: const Icon(Icons.arrow_back,
            color: Colors.white, size: 20),
      ),
    );
  }
}