import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:video_player/video_player.dart';

import '../../../home_feed/domain/entities/feed_item_entity.dart';
import '../providers/video_player_provider.dart';
import 'like_animation.dart';
import 'video_actions_bar.dart';
import 'video_info_bar.dart';

class TikTokVideoPlayer extends ConsumerStatefulWidget {
  const TikTokVideoPlayer({
    super.key,
    required this.item,
    required this.isActive,
    required this.feedType,
    this.onCommentTap,
    this.onShareTap,
    this.onSoundTap,
    this.onUsernameTap,
    this.onHashtagTap,
  });

  final FeedItemEntity item;
  final bool isActive;
  final String feedType;
  final VoidCallback? onCommentTap;
  final VoidCallback? onShareTap;
  final VoidCallback? onSoundTap;
  final VoidCallback? onUsernameTap;
  final void Function(String tag)? onHashtagTap;

  @override
  ConsumerState<TikTokVideoPlayer> createState() => _TikTokVideoPlayerState();
}

class _TikTokVideoPlayerState extends ConsumerState<TikTokVideoPlayer>
    with SingleTickerProviderStateMixin {
  late final LikeAnimationController _likeAnimCtrl;
  late final AnimationController _playIconCtrl;
  late final Animation<double> _playIconOpacity;

  @override
  void initState() {
    super.initState();
    _likeAnimCtrl = LikeAnimationController();

    _playIconCtrl = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 600),
    );
    _playIconOpacity = CurvedAnimation(
      parent: _playIconCtrl,
      curve: Curves.easeOut,
    );

    WidgetsBinding.instance.addPostFrameCallback((_) => _initPlayer());
  }

  @override
  void didUpdateWidget(TikTokVideoPlayer old) {
    super.didUpdateWidget(old);
    if (widget.isActive != old.isActive) {
      _syncPlayback();
    }
    if (widget.item.videoId != old.item.videoId) {
      _initPlayer();
    }
  }

  Future<void> _initPlayer() async {
    final notifier =
        ref.read(videoPlayerProvider(widget.item.videoId).notifier);
    final url = widget.item.hlsUrl.isNotEmpty
        ? widget.item.hlsUrl
        : widget.item.videoUrl;
    await notifier.init(url);
    _syncPlayback();
  }

  void _syncPlayback() {
    final notifier =
        ref.read(videoPlayerProvider(widget.item.videoId).notifier);
    if (widget.isActive) {
      notifier.play();
    } else {
      notifier.pause();
    }
  }

  void _onSingleTap() {
    final notifier =
        ref.read(videoPlayerProvider(widget.item.videoId).notifier);
    notifier.togglePlayPause();
    _flashPlayIcon();
  }

  void _flashPlayIcon() {
    _playIconCtrl.forward(from: 0).then((_) {
      Future.delayed(const Duration(milliseconds: 250), () {
        if (mounted) _playIconCtrl.reverse();
      });
    });
  }

  void _onDoubleTap(Offset localPosition) {
    _likeAnimCtrl.trigger(localPosition);
  }

  @override
  void dispose() {
    _playIconCtrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final playerState =
        ref.watch(videoPlayerProvider(widget.item.videoId));

    return Stack(
      fit: StackFit.expand,
      children: [
        // ── Video / thumbnail surface ─────────────────────────────────────
        _VideoSurface(
          playerState: playerState,
          thumbnailUrl: widget.item.thumbnailUrl,
        ),

        // ── Gesture detector ──────────────────────────────────────────────
        _GestureLayer(
          onSingleTap: _onSingleTap,
          onDoubleTap: _onDoubleTap,
        ),

        // ── Top + bottom gradient overlays ────────────────────────────────
        const _GradientOverlays(),

        // ── Play / pause flash overlay ────────────────────────────────────
        Center(
          child: FadeTransition(
            opacity: _playIconOpacity,
            child: Container(
              width: 72,
              height: 72,
              decoration: const BoxDecoration(
                color: Colors.black38,
                shape: BoxShape.circle,
              ),
              child: Icon(
                playerState.isPlaying ? Icons.pause : Icons.play_arrow,
                color: Colors.white,
                size: 40,
              ),
            ),
          ),
        ),

        // ── Double-tap heart animations ───────────────────────────────────
        LikeAnimationOverlay(controller: _likeAnimCtrl),

        // ── Right-edge action column ──────────────────────────────────────
        VideoActionsBar(
          item: widget.item,
          isPlaying: playerState.isPlaying,
          feedType: widget.feedType,
          onCommentTap: widget.onCommentTap,
          onShareTap: widget.onShareTap,
          onSoundTap: widget.onSoundTap,
          onUsernameTap: widget.onUsernameTap,  // ← ADDED
        ),

        // ── Bottom-left info bar ──────────────────────────────────────────
        Positioned(
          left: 0,
          right: 0,
          bottom: 0,
          child: VideoInfoBar(
            item: widget.item,
            onUsernameTap: widget.onUsernameTap,
            onHashtagTap: widget.onHashtagTap,
          ),
        ),
      ],
    );
  }
}

class _VideoSurface extends StatelessWidget {
  const _VideoSurface({
    required this.playerState,
    required this.thumbnailUrl,
  });

  final VideoPlayerState playerState;
  final String thumbnailUrl;

  @override
  Widget build(BuildContext context) {
    if (!playerState.isInitialized || playerState.controller == null) {
      return Image.network(
        thumbnailUrl,
        fit: BoxFit.cover,
        width: double.infinity,
        height: double.infinity,
        errorBuilder: (_, __, ___) => const ColoredBox(color: Colors.black),
      );
    }

    return FittedBox(
      fit: BoxFit.cover,
      child: SizedBox(
        width: playerState.controller!.value.size.width,
        height: playerState.controller!.value.size.height,
        child: VideoPlayer(playerState.controller!),
      ),
    );
  }
}

class _GestureLayer extends StatelessWidget {
  const _GestureLayer({
    required this.onSingleTap,
    required this.onDoubleTap,
  });

  final VoidCallback onSingleTap;
  final void Function(Offset) onDoubleTap;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      behavior: HitTestBehavior.translucent,
      onTap: onSingleTap,
      onDoubleTapDown: (details) => onDoubleTap(details.localPosition),
      onDoubleTap: () {},
      child: const SizedBox.expand(),
    );
  }
}

class _GradientOverlays extends StatelessWidget {
  const _GradientOverlays();

  @override
  Widget build(BuildContext context) {
    return Stack(
      children: [
        Positioned(
          top: 0,
          left: 0,
          right: 0,
          height: 100,
          child: DecoratedBox(
            decoration: BoxDecoration(
              gradient: LinearGradient(
                begin: Alignment.topCenter,
                end: Alignment.bottomCenter,
                colors: [
                  Colors.black.withValues(alpha: 0.45),
                  Colors.transparent,
                ],
              ),
            ),
          ),
        ),
        Positioned(
          bottom: 0,
          left: 0,
          right: 0,
          height: 200,
          child: DecoratedBox(
            decoration: BoxDecoration(
              gradient: LinearGradient(
                begin: Alignment.bottomCenter,
                end: Alignment.topCenter,
                colors: [
                  Colors.black.withValues(alpha: 0.75),
                  Colors.transparent,
                ],
              ),
            ),
          ),
        ),
      ],
    );
  }
}