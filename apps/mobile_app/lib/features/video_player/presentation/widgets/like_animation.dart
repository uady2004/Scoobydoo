import 'package:flutter/material.dart';

// ─────────────────────────────────────────────────────────────────────────────
// Single floating heart widget
// ─────────────────────────────────────────────────────────────────────────────

/// Renders one heart at [position] with a scale + opacity animation and calls
/// [onDone] when the 700 ms sequence finishes.
class _HeartOverlay extends StatefulWidget {
  const _HeartOverlay({
    super.key,
    required this.position,
    required this.onDone,
  });

  final Offset position;
  final VoidCallback onDone;

  @override
  State<_HeartOverlay> createState() => _HeartOverlayState();
}

class _HeartOverlayState extends State<_HeartOverlay>
    with SingleTickerProviderStateMixin {
  late final AnimationController _ctrl;
  late final Animation<double> _scale;
  late final Animation<double> _opacity;

  @override
  void initState() {
    super.initState();
    _ctrl = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 700),
    );

    // Scale: 0 → 1.5 (elastic pop) → 1.0 → 0 (fade out shrink)
    _scale = TweenSequence<double>([
      TweenSequenceItem(
        tween: Tween(begin: 0.0, end: 1.5)
            .chain(CurveTween(curve: Curves.elasticOut)),
        weight: 50,
      ),
      TweenSequenceItem(
        tween: Tween(begin: 1.5, end: 1.0)
            .chain(CurveTween(curve: Curves.easeOut)),
        weight: 15,
      ),
      TweenSequenceItem(
        tween: Tween(begin: 1.0, end: 0.0)
            .chain(CurveTween(curve: Curves.easeIn)),
        weight: 35,
      ),
    ]).animate(_ctrl);

    // Opacity: 0 → 1 quickly, hold, then 1 → 0
    _opacity = TweenSequence<double>([
      TweenSequenceItem(tween: Tween(begin: 0.0, end: 1.0), weight: 15),
      TweenSequenceItem(tween: ConstantTween(1.0), weight: 50),
      TweenSequenceItem(tween: Tween(begin: 1.0, end: 0.0), weight: 35),
    ]).animate(_ctrl);

    _ctrl.forward().then((_) => widget.onDone());
  }

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    // Centre the 80 px icon on the tap position.
    return Positioned(
      left: widget.position.dx - 40,
      top: widget.position.dy - 40,
      child: AnimatedBuilder(
        animation: _ctrl,
        builder: (_, __) => Opacity(
          opacity: _opacity.value.clamp(0.0, 1.0),
          child: Transform.scale(
            scale: _scale.value.clamp(0.0, 2.0),
            child: const Icon(
              Icons.favorite,
              color: Colors.red,
              size: 80,
              shadows: [
                Shadow(color: Colors.black45, blurRadius: 16),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Overlay widget — manages a live list of hearts
// ─────────────────────────────────────────────────────────────────────────────

/// Drop anywhere inside a [Stack].
/// Call [LikeAnimationController.trigger] with the tap [Offset] to spawn a
/// heart.  Multiple rapid taps each spawn their own heart simultaneously.
class LikeAnimationOverlay extends StatefulWidget {
  const LikeAnimationOverlay({super.key, required this.controller});

  final LikeAnimationController controller;

  @override
  State<LikeAnimationOverlay> createState() => _LikeAnimationOverlayState();
}

class _LikeAnimationOverlayState extends State<LikeAnimationOverlay> {
  final List<_HeartEntry> _hearts = [];

  @override
  void initState() {
    super.initState();
    widget.controller._attach(this);
  }

  @override
  void dispose() {
    widget.controller._detach();
    super.dispose();
  }

  void _spawn(Offset position) {
    if (!mounted) return;
    setState(() => _hearts.add(_HeartEntry(position: position)));
  }

  void _remove(_HeartEntry entry) {
    if (!mounted) return;
    setState(() => _hearts.remove(entry));
  }

  @override
  Widget build(BuildContext context) {
    return IgnorePointer(
      child: Stack(
        children: _hearts
            .map((e) => _HeartOverlay(
                  key: ValueKey(e.id),
                  position: e.position,
                  onDone: () => _remove(e),
                ))
            .toList(),
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Entry model and controller
// ─────────────────────────────────────────────────────────────────────────────

class _HeartEntry {
  _HeartEntry({required this.position}) : id = _idCounter++;
  static int _idCounter = 0;
  final int id;
  final Offset position;
}

/// Decouples the GestureDetector from the overlay widget tree.
/// Attach one to a [TikTokVideoPlayer] and forward double-tap positions.
class LikeAnimationController {
  _LikeAnimationOverlayState? _state;

  void _attach(_LikeAnimationOverlayState state) => _state = state;
  void _detach() => _state = null;

  /// Spawn a heart centred on [tapPosition] in the overlay's local
  /// coordinate space.
  void trigger(Offset tapPosition) => _state?._spawn(tapPosition);
}
