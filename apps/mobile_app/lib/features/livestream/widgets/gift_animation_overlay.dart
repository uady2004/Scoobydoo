import 'package:flutter/material.dart';
import 'package:lottie/lottie.dart';

import '../models/livestream_model.dart';

/// Displays the gift animation that pops up when a gift is sent.
/// Renders above the video player for a fixed duration then dismisses.
class GiftAnimationOverlay extends StatefulWidget {
  const GiftAnimationOverlay({
    super.key,
    required this.gift,
    required this.onDismiss,
  });

  final GiftSentEvent gift;
  final VoidCallback onDismiss;

  @override
  State<GiftAnimationOverlay> createState() => _GiftAnimationOverlayState();
}

class _GiftAnimationOverlayState extends State<GiftAnimationOverlay>
    with SingleTickerProviderStateMixin {
  late final AnimationController _controller;
  late final Animation<double> _opacity;
  late final Animation<Offset> _slide;

  @override
  void initState() {
    super.initState();
    _controller = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 3000),
    );

    _opacity = TweenSequence<double>([
      TweenSequenceItem(tween: Tween(begin: 0.0, end: 1.0), weight: 10),
      TweenSequenceItem(tween: ConstantTween(1.0), weight: 75),
      TweenSequenceItem(tween: Tween(begin: 1.0, end: 0.0), weight: 15),
    ]).animate(_controller);

    _slide = TweenSequence<Offset>([
      TweenSequenceItem(
        tween: Tween(begin: const Offset(0, 0.3), end: Offset.zero)
            .chain(CurveTween(curve: Curves.easeOut)),
        weight: 15,
      ),
      TweenSequenceItem(tween: ConstantTween(Offset.zero), weight: 85),
    ]).animate(_controller);

    _controller.forward().then((_) => widget.onDismiss());
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Positioned(
      left: 16,
      bottom: 140,
      child: AnimatedBuilder(
        animation: _controller,
        builder: (_, child) => FadeTransition(
          opacity: _opacity,
          child: SlideTransition(position: _slide, child: child),
        ),
        child: _buildCard(),
      ),
    );
  }

  Widget _buildCard() {
    final isLottie = widget.gift.animationUrl.endsWith('.json');
    return Container(
      width: 200,
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      decoration: BoxDecoration(
        gradient: const LinearGradient(
          colors: [Color(0xFFFF2D55), Color(0xFFFF6B35)],
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
        ),
        borderRadius: BorderRadius.circular(16),
        boxShadow: const [
          BoxShadow(
            color: Colors.black38,
            blurRadius: 8,
            offset: Offset(0, 4),
          )
        ],
      ),
      child: Row(
        children: [
          // Gift icon / Lottie animation
          SizedBox(
            width: 56,
            height: 56,
            child: isLottie && widget.gift.animationUrl.isNotEmpty
                ? Lottie.network(
                    widget.gift.animationUrl,
                    fit: BoxFit.contain,
                    repeat: true,
                    errorBuilder: (_, __, ___) =>
                        const Icon(Icons.card_giftcard, color: Colors.white, size: 40),
                  )
                : const Icon(Icons.card_giftcard, color: Colors.white, size: 40),
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  widget.gift.senderName,
                  style: const TextStyle(
                    color: Colors.white,
                    fontWeight: FontWeight.bold,
                    fontSize: 12,
                  ),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
                const SizedBox(height: 2),
                Text(
                  'sent ${widget.gift.giftName}',
                  style: const TextStyle(color: Colors.white70, fontSize: 11),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
                if (widget.gift.comboCount > 1) ...[
                  const SizedBox(height: 2),
                  _ComboLabel(count: widget.gift.comboCount),
                ],
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _ComboLabel extends StatefulWidget {
  const _ComboLabel({required this.count});
  final int count;

  @override
  State<_ComboLabel> createState() => _ComboLabelState();
}

class _ComboLabelState extends State<_ComboLabel>
    with SingleTickerProviderStateMixin {
  late final AnimationController _ctrl;
  late final Animation<double> _scale;

  @override
  void initState() {
    super.initState();
    _ctrl = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 300),
    )..forward();
    _scale = CurvedAnimation(parent: _ctrl, curve: Curves.elasticOut);
  }

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return ScaleTransition(
      scale: _scale,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
        decoration: BoxDecoration(
          color: Colors.white.withValues(alpha: 0.25),
          borderRadius: BorderRadius.circular(10),
        ),
        child: Text(
          'x${widget.count} COMBO!',
          style: const TextStyle(
            color: Colors.white,
            fontWeight: FontWeight.bold,
            fontSize: 11,
          ),
        ),
      ),
    );
  }
}
