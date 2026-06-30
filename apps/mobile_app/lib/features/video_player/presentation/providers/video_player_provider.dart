import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:video_player/video_player.dart';

// ─────────────────────────────────────────────────────────────────────────────
// State
// ─────────────────────────────────────────────────────────────────────────────

class VideoPlayerState {
  const VideoPlayerState({
    this.controller,
    this.isInitialized = false,
    this.isPlaying = false,
    this.isMuted = false,
    this.error,
  });

  final VideoPlayerController? controller;
  final bool isInitialized;
  final bool isPlaying;
  final bool isMuted;
  final String? error;

  VideoPlayerState copyWith({
    VideoPlayerController? controller,
    bool? isInitialized,
    bool? isPlaying,
    bool? isMuted,
    String? error,
    bool clearError = false,
  }) {
    return VideoPlayerState(
      controller: controller ?? this.controller,
      isInitialized: isInitialized ?? this.isInitialized,
      isPlaying: isPlaying ?? this.isPlaying,
      isMuted: isMuted ?? this.isMuted,
      error: clearError ? null : (error ?? this.error),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Notifier
// ─────────────────────────────────────────────────────────────────────────────

/// Per-video StateNotifier.  One instance is created per [videoId] via the
/// [videoPlayerProvider] family provider.
///
/// Lifecycle:
///   1. Widget mounts  → calls [init] with the HLS URL.
///   2. Widget enters viewport → [play].
///   3. Widget leaves viewport → [pause].
///   4. Widget is disposed → Riverpod disposes the notifier → [dispose]
///      releases the [VideoPlayerController].
class VideoPlayerNotifier extends StateNotifier<VideoPlayerState> {
  VideoPlayerNotifier() : super(const VideoPlayerState());

  VideoPlayerController? _controller;

  /// Initialises the player with the provided [hlsUrl].
  /// Safe to call multiple times; subsequent calls are no-ops if already
  /// initialised with the same controller.
  Future<void> init(String hlsUrl) async {
    if (state.isInitialized) return;

    try {
      final controller = VideoPlayerController.networkUrl(
        Uri.parse(hlsUrl),
        videoPlayerOptions: VideoPlayerOptions(mixWithOthers: true),
      );
      _controller = controller;
      state = state.copyWith(controller: controller, clearError: true);

      await controller.initialize();
      await controller.setLooping(true);
      await controller.setVolume(state.isMuted ? 0.0 : 1.0);

      // Listen for external state changes (e.g. OS media controls).
      controller.addListener(_onControllerUpdate);

      state = state.copyWith(isInitialized: true);
    } catch (e) {
      state = state.copyWith(error: e.toString(), clearError: false);
    }
  }

  void _onControllerUpdate() {
    final ctrl = _controller;
    if (ctrl == null) return;
    if (!mounted) return;
    state = state.copyWith(isPlaying: ctrl.value.isPlaying);
  }

  /// Starts playback. Does nothing if not yet initialised.
  Future<void> play() async {
    final ctrl = _controller;
    if (ctrl == null || !state.isInitialized) return;
    await ctrl.play();
    state = state.copyWith(isPlaying: true);
  }

  /// Pauses playback. Does nothing if not yet initialised.
  Future<void> pause() async {
    final ctrl = _controller;
    if (ctrl == null || !state.isInitialized) return;
    await ctrl.pause();
    state = state.copyWith(isPlaying: false);
  }

  /// Toggles play / pause.
  Future<void> togglePlayPause() async {
    if (state.isPlaying) {
      await pause();
    } else {
      await play();
    }
  }

  /// Toggles mute.
  Future<void> toggleMute() async {
    final ctrl = _controller;
    if (ctrl == null) return;
    final muted = !state.isMuted;
    await ctrl.setVolume(muted ? 0.0 : 1.0);
    state = state.copyWith(isMuted: muted);
  }

  /// Seeks to the given position.
  Future<void> seekTo(Duration position) async {
    await _controller?.seekTo(position);
  }

  /// Returns the current playback position. Returns [Duration.zero] if the
  /// controller is not yet ready.
  Duration get position => _controller?.value.position ?? Duration.zero;

  /// Returns the total duration. Returns [Duration.zero] if unknown.
  Duration get totalDuration =>
      _controller?.value.duration ?? Duration.zero;

  @override
  void dispose() {
    _controller?.removeListener(_onControllerUpdate);
    _controller?.dispose();
    _controller = null;
    super.dispose();
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Provider
// ─────────────────────────────────────────────────────────────────────────────

/// Family provider keyed by videoId.
/// The notifier (and its [VideoPlayerController]) is automatically disposed
/// when all widgets that read this provider are removed from the tree.
final videoPlayerProvider = StateNotifierProvider.family
    .autoDispose<VideoPlayerNotifier, VideoPlayerState, String>(
  (ref, videoId) => VideoPlayerNotifier(),
);
