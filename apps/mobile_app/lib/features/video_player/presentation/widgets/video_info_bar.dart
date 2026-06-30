import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';

import '../../../home_feed/domain/entities/feed_item_entity.dart';

// ─────────────────────────────────────────────────────────────────────────────
// Hashtag-aware rich text
// ─────────────────────────────────────────────────────────────────────────────

/// Renders plain text with embedded #hashtags highlighted in white bold.
/// Tapping a hashtag calls [onHashtagTap] with the tag string.
class HashtagText extends StatelessWidget {
  const HashtagText({
    super.key,
    required this.text,
    required this.style,
    this.onHashtagTap,
    this.maxLines,
    this.overflow,
  });

  final String text;
  final TextStyle style;
  final void Function(String hashtag)? onHashtagTap;
  final int? maxLines;
  final TextOverflow? overflow;

  @override
  Widget build(BuildContext context) {
    final spans = <TextSpan>[];
    final pattern = RegExp(r'(#\w+)');
    int last = 0;

    for (final match in pattern.allMatches(text)) {
      if (match.start > last) {
        spans.add(TextSpan(text: text.substring(last, match.start)));
      }
      final tag = match.group(0)!;
      spans.add(TextSpan(
        text: tag,
        style: style.copyWith(
          color: Colors.white,
          fontWeight: FontWeight.bold,
        ),
        recognizer: TapGestureRecognizer()
          ..onTap = () => onHashtagTap?.call(tag),
      ));
      last = match.end;
    }
    if (last < text.length) {
      spans.add(TextSpan(text: text.substring(last)));
    }

    return RichText(
      text: TextSpan(style: style, children: spans),
      maxLines: maxLines,
      overflow: overflow ?? TextOverflow.clip,
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Marquee (horizontally scrolling) text
// ─────────────────────────────────────────────────────────────────────────────

class _MarqueeText extends StatefulWidget {
  const _MarqueeText({required this.text, required this.style});

  final String text;
  final TextStyle style;

  @override
  State<_MarqueeText> createState() => _MarqueeTextState();
}

class _MarqueeTextState extends State<_MarqueeText>
    with SingleTickerProviderStateMixin {
  late final ScrollController _scroll;
  late final AnimationController _ctrl;

  @override
  void initState() {
    super.initState();
    _scroll = ScrollController();
    _ctrl = AnimationController(
      vsync: this,
      duration: const Duration(seconds: 8),
    )..addStatusListener((status) {
        if (status == AnimationStatus.completed) {
          _scroll.jumpTo(0);
          _ctrl.forward(from: 0);
        }
      });

    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (_scroll.hasClients &&
          _scroll.position.maxScrollExtent > 0) {
        _ctrl.forward();
        _ctrl.addListener(() {
          if (!_scroll.hasClients) return;
          final max = _scroll.position.maxScrollExtent;
          _scroll.jumpTo(_ctrl.value * max);
        });
      }
    });
  }

  @override
  void dispose() {
    _ctrl.dispose();
    _scroll.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return SingleChildScrollView(
      controller: _scroll,
      scrollDirection: Axis.horizontal,
      physics: const NeverScrollableScrollPhysics(),
      child: Text(widget.text, style: widget.style, maxLines: 1),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// VideoInfoBar
// ─────────────────────────────────────────────────────────────────────────────

class VideoInfoBar extends StatefulWidget {
  const VideoInfoBar({
    super.key,
    required this.item,
    this.onUsernameTap,
    this.onHashtagTap,
  });

  final FeedItemEntity item;
  final VoidCallback? onUsernameTap;
  final void Function(String tag)? onHashtagTap;

  @override
  State<VideoInfoBar> createState() => _VideoInfoBarState();
}

class _VideoInfoBarState extends State<VideoInfoBar> {
  bool _expanded = false;

  @override
  Widget build(BuildContext context) {
    const baseStyle = TextStyle(
      color: Color(0xDDFFFFFF),
      fontSize: 13,
      height: 1.4,
      shadows: [Shadow(color: Colors.black54, blurRadius: 4)],
    );

    final soundLabel = widget.item.soundTitle.isNotEmpty
        ? '${widget.item.soundTitle} · @${widget.item.soundArtist.isNotEmpty ? widget.item.soundArtist : widget.item.creatorUsername}'
        : 'Original Sound · @${widget.item.creatorUsername}';

    return Padding(
      padding: const EdgeInsets.fromLTRB(12, 0, 72, 24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          // ── Username + verified badge ──────────────────────────────────
          GestureDetector(
            onTap: widget.onUsernameTap,
            behavior: HitTestBehavior.opaque,
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  '@${widget.item.creatorUsername}',
                  style: const TextStyle(
                    color: Colors.white,
                    fontWeight: FontWeight.w700,
                    fontSize: 15,
                    shadows: [Shadow(color: Colors.black54, blurRadius: 4)],
                  ),
                ),
                if (widget.item.isCreatorVerified) ...[
                  const SizedBox(width: 4),
                  const _VerifiedBadge(),
                ],
              ],
            ),
          ),
          const SizedBox(height: 6),

          // ── Description + hashtags with expand/collapse ────────────────
          if (widget.item.description.isNotEmpty) ...[
            GestureDetector(
              onTap: () => setState(() => _expanded = !_expanded),
              behavior: HitTestBehavior.opaque,
              child: _expanded
                  ? HashtagText(
                      text: widget.item.description,
                      style: baseStyle,
                      onHashtagTap: widget.onHashtagTap,
                    )
                  : Row(
                      crossAxisAlignment: CrossAxisAlignment.end,
                      children: [
                        Expanded(
                          child: HashtagText(
                            text: widget.item.description,
                            style: baseStyle,
                            maxLines: 2,
                            overflow: TextOverflow.ellipsis,
                            onHashtagTap: widget.onHashtagTap,
                          ),
                        ),
                        const SizedBox(width: 4),
                        Text(
                          '...more',
                          style: baseStyle.copyWith(
                            fontWeight: FontWeight.bold,
                            color: Colors.white,
                          ),
                        ),
                      ],
                    ),
            ),
            const SizedBox(height: 8),
          ],

          // ── Sound strip ────────────────────────────────────────────────
          Row(
            children: [
              const Icon(
                Icons.music_note,
                color: Colors.white,
                size: 14,
                shadows: [Shadow(color: Colors.black54, blurRadius: 4)],
              ),
              const SizedBox(width: 4),
              Expanded(
                child: soundLabel.length > 30
                    ? _MarqueeText(text: soundLabel, style: baseStyle)
                    : Text(soundLabel, style: baseStyle, maxLines: 1),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Verified badge
// ─────────────────────────────────────────────────────────────────────────────

class _VerifiedBadge extends StatelessWidget {
  const _VerifiedBadge();

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 16,
      height: 16,
      decoration: const BoxDecoration(
        color: Color(0xFF20D5EC),
        shape: BoxShape.circle,
      ),
      child: const Icon(Icons.check, color: Colors.white, size: 10),
    );
  }
}
