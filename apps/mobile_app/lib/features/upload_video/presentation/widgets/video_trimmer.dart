import 'package:flutter/material.dart';

// ─────────────────────────────────────────────────────────────────────────────
// Constants
// ─────────────────────────────────────────────────────────────────────────────

const _kRed = Color(0xFFEE1D52);
const _kHandleWidth = 12.0;
const _kStripHeight = 60.0;
const _kThumbnailCount = 20;

// ─────────────────────────────────────────────────────────────────────────────
// VideoTrimmer
// ─────────────────────────────────────────────────────────────────────────────

/// A horizontal trim-bar widget.
///
/// Displays a filmstrip of placeholder thumbnail cells with draggable red
/// handles at each end. Calls [onTrimChanged] whenever either handle moves.
class VideoTrimmer extends StatefulWidget {
  const VideoTrimmer({
    super.key,
    required this.totalDurationSeconds,
    required this.onTrimChanged,
  });

  /// Total duration of the source video in seconds.
  final int totalDurationSeconds;

  /// Called with updated (startFraction, endFraction) whenever a handle moves.
  final void Function(double start, double end) onTrimChanged;

  @override
  State<VideoTrimmer> createState() => _VideoTrimmerState();
}

class _VideoTrimmerState extends State<VideoTrimmer> {
  double _startFraction = 0.0;
  double _endFraction = 1.0;

  // ── Helpers ────────────────────────────────────────────────────────────────

  String _formatSeconds(double seconds) {
    final s = seconds.toInt().clamp(0, widget.totalDurationSeconds);
    final m = s ~/ 60;
    final rem = s % 60;
    return '${m.toString().padLeft(1, '0')}:${rem.toString().padLeft(2, '0')}';
  }

  void _updateStart(double delta, double totalWidth) {
    final newFraction = (_startFraction + delta / totalWidth).clamp(
      0.0,
      _endFraction - 0.1,
    );
    setState(() => _startFraction = newFraction);
    widget.onTrimChanged(_startFraction, _endFraction);
  }

  void _updateEnd(double delta, double totalWidth) {
    final newFraction = (_endFraction + delta / totalWidth).clamp(
      _startFraction + 0.1,
      1.0,
    );
    setState(() => _endFraction = newFraction);
    widget.onTrimChanged(_startFraction, _endFraction);
  }

  // ── Build ──────────────────────────────────────────────────────────────────

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      height: 80,
      child: LayoutBuilder(
        builder: (context, constraints) {
          final totalWidth = constraints.maxWidth;
          final startPx = _startFraction * totalWidth;
          final endPx = _endFraction * totalWidth;

          return Column(
            children: [
              // Trim strip
              SizedBox(
                height: _kStripHeight,
                child: Stack(
                  clipBehavior: Clip.none,
                  children: [
                    // Thumbnail strip
                    Row(
                      children: List.generate(_kThumbnailCount, (i) {
                        return Container(
                          width: totalWidth / _kThumbnailCount - 1,
                          height: _kStripHeight,
                          margin:
                              const EdgeInsets.symmetric(horizontal: 0.5),
                          decoration: BoxDecoration(
                            color: Colors.grey[800],
                            borderRadius: BorderRadius.circular(2),
                          ),
                        );
                      }),
                    ),

                    // Active-range top indicator line
                    Positioned(
                      top: 0,
                      left: startPx + _kHandleWidth,
                      width: (endPx - startPx - _kHandleWidth * 2)
                          .clamp(0.0, double.infinity),
                      height: 2,
                      child: Container(
                        color: _kRed.withValues(alpha: 0.4),
                      ),
                    ),

                    // Start handle
                    Positioned(
                      left: startPx,
                      top: 0,
                      child: GestureDetector(
                        behavior: HitTestBehavior.opaque,
                        onPanUpdate: (details) =>
                            _updateStart(details.delta.dx, totalWidth),
                        child: Container(
                          width: _kHandleWidth,
                          height: _kStripHeight,
                          decoration: const BoxDecoration(
                            color: _kRed,
                            borderRadius: BorderRadius.only(
                              topLeft: Radius.circular(4),
                              bottomLeft: Radius.circular(4),
                            ),
                          ),
                          child: const Center(
                            child: _HandleGrip(),
                          ),
                        ),
                      ),
                    ),

                    // End handle
                    Positioned(
                      left: endPx - _kHandleWidth,
                      top: 0,
                      child: GestureDetector(
                        behavior: HitTestBehavior.opaque,
                        onPanUpdate: (details) =>
                            _updateEnd(details.delta.dx, totalWidth),
                        child: Container(
                          width: _kHandleWidth,
                          height: _kStripHeight,
                          decoration: const BoxDecoration(
                            color: _kRed,
                            borderRadius: BorderRadius.only(
                              topRight: Radius.circular(4),
                              bottomRight: Radius.circular(4),
                            ),
                          ),
                          child: const Center(
                            child: _HandleGrip(),
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
              ),

              // Time labels
              const SizedBox(height: 4),
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Text(
                    _formatSeconds(
                        widget.totalDurationSeconds * _startFraction),
                    style: const TextStyle(
                        color: Colors.grey, fontSize: 11),
                  ),
                  Text(
                    _formatSeconds(
                        widget.totalDurationSeconds * _endFraction),
                    style: const TextStyle(
                        color: Colors.grey, fontSize: 11),
                  ),
                ],
              ),
            ],
          );
        },
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Handle grip indicator (three short white lines)
// ─────────────────────────────────────────────────────────────────────────────

class _HandleGrip extends StatelessWidget {
  const _HandleGrip();

  @override
  Widget build(BuildContext context) {
    return Column(
      mainAxisSize: MainAxisSize.min,
      mainAxisAlignment: MainAxisAlignment.center,
      children: List.generate(
        3,
        (_) => Container(
          width: 2,
          height: 10,
          margin: const EdgeInsets.symmetric(vertical: 1.5),
          decoration: BoxDecoration(
            color: Colors.white.withValues(alpha: 0.8),
            borderRadius: BorderRadius.circular(1),
          ),
        ),
      ),
    );
  }
}
