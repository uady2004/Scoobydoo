import 'dart:math' as math;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:share_plus/share_plus.dart';

// ---------------------------------------------------------------------------
// Data model & provider
// ---------------------------------------------------------------------------

class SoundData {
  final String id;
  final String title;
  final String artist;
  final String artworkUrl;
  final Duration duration;
  final int usageCount;
  final int likeCount;
  final List<Map<String, dynamic>> videos;

  const SoundData({
    required this.id,
    required this.title,
    required this.artist,
    required this.artworkUrl,
    required this.duration,
    this.usageCount = 0,
    this.likeCount = 0,
    this.videos = const [],
  });
}

final _soundProvider =
    FutureProvider.family<SoundData, String>((ref, soundId) async {
  await Future<void>.delayed(const Duration(milliseconds: 500));
  return SoundData(
    id: soundId,
    title: 'Original Sound',
    artist: '@creator_name',
    artworkUrl: '',
    duration: const Duration(seconds: 43),
    usageCount: 284700,
    likeCount: 51200,
    videos: List.generate(18, (i) => {
      'id': 'v$i',
      'thumbnail_url': '',
      'view_count': (i + 1) * 23400,
    }),
  );
});

// ---------------------------------------------------------------------------
// Animated waveform visualiser
// ---------------------------------------------------------------------------

class _Waveform extends StatefulWidget {
  final bool isPlaying;

  const _Waveform({required this.isPlaying});

  @override
  State<_Waveform> createState() => _WaveformState();
}

class _WaveformState extends State<_Waveform>
    with SingleTickerProviderStateMixin {
  late final AnimationController _ctrl;
  final _rng = math.Random(42);
  late final List<double> _baseHeights;

  @override
  void initState() {
    super.initState();
    _baseHeights = List.generate(20, (_) => 0.2 + _rng.nextDouble() * 0.8);
    _ctrl = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 900),
    )..repeat(reverse: true);
  }

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: _ctrl,
      builder: (_, __) {
        return Row(
          mainAxisAlignment: MainAxisAlignment.center,
          crossAxisAlignment: CrossAxisAlignment.center,
          children: List.generate(_baseHeights.length, (i) {
            final base = _baseHeights[i];
            final height = widget.isPlaying
                ? base *
                    (0.4 +
                        0.6 *
                            math.sin(
                                _ctrl.value * math.pi * 2 + i * 0.5).abs())
                : 0.15;
            final maxH = 36.0;

            return Container(
              width: 4,
              height: (maxH * height).clamp(4.0, maxH),
              margin: const EdgeInsets.symmetric(horizontal: 2),
              decoration: BoxDecoration(
                color: widget.isPlaying
                    ? const Color(0xFFFF0050)
                    : Colors.white24,
                borderRadius: BorderRadius.circular(2),
              ),
            );
          }),
        );
      },
    );
  }
}

// ---------------------------------------------------------------------------
// Screen
// ---------------------------------------------------------------------------

class SoundScreen extends ConsumerStatefulWidget {
  final String soundId;

  const SoundScreen({super.key, required this.soundId});

  @override
  ConsumerState<SoundScreen> createState() => _SoundScreenState();
}

class _SoundScreenState extends ConsumerState<SoundScreen> {
  bool _isPlaying = false;
  bool _isLiked = false;
  Duration _position = Duration.zero;

  String _fmtDuration(Duration d) {
    final m = d.inMinutes.remainder(60).toString().padLeft(2, '0');
    final s = d.inSeconds.remainder(60).toString().padLeft(2, '0');
    return '$m:$s';
  }

  String _fmtCount(int n) {
    if (n >= 1000000) return '${(n / 1000000).toStringAsFixed(1)}M';
    if (n >= 1000) return '${(n / 1000).toStringAsFixed(1)}K';
    return '$n';
  }

  @override
  Widget build(BuildContext context) {
    final asyncData = ref.watch(_soundProvider(widget.soundId));

    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        backgroundColor: Colors.black,
        iconTheme: const IconThemeData(color: Colors.white),
        title: const Text(
          'Sound',
          style: TextStyle(
              color: Colors.white, fontWeight: FontWeight.w600, fontSize: 17),
        ),
        centerTitle: true,
        actions: [
          IconButton(
            icon: const Icon(Icons.more_vert, color: Colors.white),
            onPressed: () {},
          ),
        ],
      ),
      body: asyncData.when(
        loading: () => const Center(
          child: CircularProgressIndicator(color: Color(0xFFFF0050)),
        ),
        error: (e, _) => Center(
          child: Text(e.toString(),
              style: const TextStyle(color: Colors.white54)),
        ),
        data: (sound) => CustomScrollView(
          slivers: [
            // ── Sound card ──────────────────────────────────────────────
            SliverToBoxAdapter(
              child: Padding(
                padding: const EdgeInsets.fromLTRB(16, 20, 16, 8),
                child: Column(
                  children: [
                    // Album art + info row
                    Row(
                      crossAxisAlignment: CrossAxisAlignment.center,
                      children: [
                        // Album art
                        Container(
                          width: 80,
                          height: 80,
                          decoration: BoxDecoration(
                            borderRadius: BorderRadius.circular(8),
                            color: const Color(0xFF2A2A2A),
                            boxShadow: const [
                              BoxShadow(
                                color: Colors.black38,
                                blurRadius: 16,
                                offset: Offset(0, 6),
                              ),
                            ],
                          ),
                          child: sound.artworkUrl.isNotEmpty
                              ? ClipRRect(
                                  borderRadius: BorderRadius.circular(8),
                                  child: Image.network(sound.artworkUrl,
                                      fit: BoxFit.cover),
                                )
                              : const Icon(Icons.music_note,
                                  color: Color(0xFFFF0050), size: 40),
                        ),
                        const SizedBox(width: 14),
                        // Title, artist, duration
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                sound.title,
                                style: const TextStyle(
                                  color: Colors.white,
                                  fontSize: 16,
                                  fontWeight: FontWeight.w700,
                                  height: 1.2,
                                ),
                                maxLines: 2,
                                overflow: TextOverflow.ellipsis,
                              ),
                              const SizedBox(height: 4),
                              Text(
                                sound.artist,
                                style: const TextStyle(
                                  color: Colors.white54,
                                  fontSize: 13,
                                ),
                              ),
                              const SizedBox(height: 4),
                              Text(
                                _fmtDuration(sound.duration),
                                style: const TextStyle(
                                  color: Colors.white38,
                                  fontSize: 12,
                                ),
                              ),
                            ],
                          ),
                        ),
                      ],
                    ),
                    const SizedBox(height: 20),

                    // Waveform + play/pause row
                    Row(
                      children: [
                        GestureDetector(
                          onTap: () =>
                              setState(() => _isPlaying = !_isPlaying),
                          child: Container(
                            width: 44,
                            height: 44,
                            decoration: const BoxDecoration(
                              color: Color(0xFFFF0050),
                              shape: BoxShape.circle,
                            ),
                            child: Icon(
                              _isPlaying ? Icons.pause : Icons.play_arrow,
                              color: Colors.white,
                              size: 24,
                            ),
                          ),
                        ),
                        const SizedBox(width: 12),
                        Expanded(
                          child: SizedBox(
                            height: 40,
                            child: _Waveform(isPlaying: _isPlaying),
                          ),
                        ),
                      ],
                    ),

                    // Slider
                    SliderTheme(
                      data: SliderTheme.of(context).copyWith(
                        activeTrackColor: const Color(0xFFFF0050),
                        inactiveTrackColor: Colors.white12,
                        thumbColor: const Color(0xFFFF0050),
                        thumbShape: const RoundSliderThumbShape(
                            enabledThumbRadius: 6),
                        overlayShape: SliderComponentShape.noOverlay,
                        trackHeight: 2.5,
                      ),
                      child: Slider(
                        value: _position.inMilliseconds.toDouble(),
                        max: sound.duration.inMilliseconds.toDouble(),
                        onChanged: (v) => setState(
                          () => _position =
                              Duration(milliseconds: v.toInt()),
                        ),
                      ),
                    ),

                    // Stats row: usage count, like, share
                    Row(
                      children: [
                        const Icon(Icons.play_circle_outline,
                            size: 16, color: Colors.white54),
                        const SizedBox(width: 4),
                        Text(
                          '${_fmtCount(sound.usageCount)} videos',
                          style: const TextStyle(
                              color: Colors.white54, fontSize: 13),
                        ),
                        const Spacer(),
                        // Like button
                        GestureDetector(
                          onTap: () =>
                              setState(() => _isLiked = !_isLiked),
                          child: Row(
                            children: [
                              Icon(
                                _isLiked
                                    ? Icons.favorite
                                    : Icons.favorite_border,
                                size: 20,
                                color: _isLiked
                                    ? const Color(0xFFFF0050)
                                    : Colors.white54,
                              ),
                              const SizedBox(width: 4),
                              Text(
                                _fmtCount(
                                    sound.likeCount + (_isLiked ? 1 : 0)),
                                style: const TextStyle(
                                    color: Colors.white54, fontSize: 13),
                              ),
                            ],
                          ),
                        ),
                        const SizedBox(width: 16),
                        // Share button
                        GestureDetector(
                          onTap: () => SharePlus.instance.share(
                            ShareParams(
                              text:
                                  'Listen to "${sound.title}" by ${sound.artist}\nhttps://tiktok-clone.dev/sound/${sound.id}',
                            ),
                          ),
                          child: const Icon(Icons.share_outlined,
                              size: 20, color: Colors.white54),
                        ),
                      ],
                    ),
                    const SizedBox(height: 20),

                    // "Use this sound" button
                    SizedBox(
                      width: double.infinity,
                      child: DecoratedBox(
                        decoration: BoxDecoration(
                          gradient: const LinearGradient(
                            colors: [Color(0xFFEE1D52), Color(0xFFFF0050)],
                          ),
                          borderRadius: BorderRadius.circular(8),
                        ),
                        child: ElevatedButton.icon(
                          onPressed: () {
                            context.push(
                                '/camera?soundId=${widget.soundId}');
                          },
                          icon: const Icon(Icons.music_note, size: 18),
                          label: const Text(
                            'Use this sound',
                            style: TextStyle(
                                fontSize: 15, fontWeight: FontWeight.w600),
                          ),
                          style: ElevatedButton.styleFrom(
                            backgroundColor: Colors.transparent,
                            foregroundColor: Colors.white,
                            shadowColor: Colors.transparent,
                            padding: const EdgeInsets.symmetric(vertical: 14),
                            minimumSize: Size.zero,
                            shape: RoundedRectangleBorder(
                              borderRadius: BorderRadius.circular(8),
                            ),
                            elevation: 0,
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ),

            // ── Section header ──────────────────────────────────────────
            SliverToBoxAdapter(
              child: Padding(
                padding: const EdgeInsets.fromLTRB(16, 12, 16, 10),
                child: Row(
                  children: [
                    const Text(
                      'Videos with this sound',
                      style: TextStyle(
                        color: Colors.white,
                        fontWeight: FontWeight.w700,
                        fontSize: 16,
                      ),
                    ),
                    const SizedBox(width: 8),
                    Text(
                      _fmtCount(sound.videos.length),
                      style: const TextStyle(
                          color: Colors.white54, fontSize: 14),
                    ),
                  ],
                ),
              ),
            ),

            // ── Video grid (2-col, 0.6 aspect ratio) ───────────────────
            SliverPadding(
              padding: const EdgeInsets.fromLTRB(1, 0, 1, 32),
              sliver: SliverGrid(
                gridDelegate:
                    const SliverGridDelegateWithFixedCrossAxisCount(
                  crossAxisCount: 2,
                  crossAxisSpacing: 1.5,
                  mainAxisSpacing: 1.5,
                  childAspectRatio: 0.6,
                ),
                delegate: SliverChildBuilderDelegate(
                  (context, index) {
                    final video = sound.videos[index];
                    final thumbnailUrl =
                        video['thumbnail_url'] as String? ?? '';
                    final videoId = video['id'] as String? ?? '';
                    final viewCount =
                        (video['view_count'] as num?)?.toInt() ?? 0;

                    return GestureDetector(
                      onTap: () => context.push('/video/$videoId'),
                      child: Stack(
                        fit: StackFit.expand,
                        children: [
                          thumbnailUrl.isNotEmpty
                              ? Image.network(thumbnailUrl,
                                  fit: BoxFit.cover,
                                  errorBuilder: (_, __, ___) =>
                                      Container(
                                          color: const Color(0xFF2A2A2A)))
                              : Container(
                                  color: const Color(0xFF2A2A2A),
                                  child: const Icon(
                                    Icons.play_circle_outline,
                                    color: Colors.white24,
                                    size: 32,
                                  ),
                                ),
                          // View count
                          Positioned(
                            bottom: 6,
                            left: 6,
                            child: Row(
                              mainAxisSize: MainAxisSize.min,
                              children: [
                                const Icon(Icons.play_arrow,
                                    size: 13, color: Colors.white),
                                const SizedBox(width: 2),
                                Text(
                                  _fmtCount(viewCount),
                                  style: const TextStyle(
                                    color: Colors.white,
                                    fontSize: 11,
                                    fontWeight: FontWeight.w600,
                                    shadows: [
                                      Shadow(
                                          blurRadius: 4,
                                          color: Colors.black54),
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
                  childCount: sound.videos.length,
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
