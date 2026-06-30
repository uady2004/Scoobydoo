import 'dart:async';

import 'package:flutter/material.dart';

import '../models/livestream_model.dart';

/// In-stream poll card shown at the bottom of the stream view.
/// Displays live percentage bars that update as votes come in.
class PollOverlay extends StatefulWidget {
  const PollOverlay({
    super.key,
    required this.poll,
    required this.votedOptionId,
    required this.onVote,
    required this.onDismiss,
  });

  final Poll poll;
  final String? votedOptionId;
  final Future<void> Function(String pollId, String optionId) onVote;
  final VoidCallback onDismiss;

  @override
  State<PollOverlay> createState() => _PollOverlayState();
}

class _PollOverlayState extends State<PollOverlay> {
  late int _secondsLeft;
  Timer? _timer;

  @override
  void initState() {
    super.initState();
    _secondsLeft = _computeSecondsLeft();
    if (widget.poll.status == PollStatus.active && widget.poll.durationSecs > 0) {
      _startTimer();
    }
  }

  @override
  void didUpdateWidget(PollOverlay old) {
    super.didUpdateWidget(old);
    if (widget.poll.status == PollStatus.closed && _timer != null) {
      _timer?.cancel();
    }
  }

  int _computeSecondsLeft() {
    if (widget.poll.durationSecs <= 0) return 0;
    final elapsed =
        DateTime.now().difference(widget.poll.createdAt).inSeconds;
    return (widget.poll.durationSecs - elapsed).clamp(0, widget.poll.durationSecs);
  }

  void _startTimer() {
    _timer?.cancel();
    _timer = Timer.periodic(const Duration(seconds: 1), (_) {
      if (!mounted) return;
      setState(() {
        if (_secondsLeft > 0) {
          _secondsLeft--;
        } else {
          _timer?.cancel();
        }
      });
    });
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  bool get _isClosed =>
      widget.poll.status == PollStatus.closed || _secondsLeft == 0;

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.symmetric(horizontal: 12),
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: Colors.black.withValues(alpha: 0.75),
        borderRadius: BorderRadius.circular(16),
        border: Border.all(
          color: Colors.white.withValues(alpha: 0.15),
          width: 1,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          _buildHeader(),
          const SizedBox(height: 10),
          ...widget.poll.options.map((opt) => _buildOptionRow(opt)),
          const SizedBox(height: 8),
          _buildFooter(),
        ],
      ),
    );
  }

  Widget _buildHeader() {
    return Row(
      children: [
        const Icon(Icons.poll, color: Color(0xFFFF2D55), size: 16),
        const SizedBox(width: 6),
        Expanded(
          child: Text(
            widget.poll.question,
            style: const TextStyle(
              color: Colors.white,
              fontWeight: FontWeight.bold,
              fontSize: 14,
            ),
          ),
        ),
        if (!_isClosed && widget.poll.durationSecs > 0)
          _CountdownChip(secondsLeft: _secondsLeft),
        if (_isClosed)
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
            decoration: BoxDecoration(
              color: Colors.grey.shade700,
              borderRadius: BorderRadius.circular(8),
            ),
            child: const Text(
              'Closed',
              style: TextStyle(color: Colors.white70, fontSize: 11),
            ),
          ),
        const SizedBox(width: 6),
        GestureDetector(
          onTap: widget.onDismiss,
          child: const Icon(Icons.close, color: Colors.white54, size: 18),
        ),
      ],
    );
  }

  Widget _buildOptionRow(PollOption opt) {
    final isVoted = widget.votedOptionId == opt.id;
    final total = widget.poll.totalVotes;
    final fraction =
        total > 0 ? (opt.voteCount / total).clamp(0.0, 1.0) : 0.0;
    final percent = (fraction * 100).round();

    final bool canVote =
        !_isClosed && widget.votedOptionId == null;

    return GestureDetector(
      onTap: canVote ? () => widget.onVote(widget.poll.id, opt.id) : null,
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 4),
        child: Stack(
          alignment: Alignment.centerLeft,
          children: [
            // Progress bar background
            Container(
              height: 36,
              decoration: BoxDecoration(
                color: Colors.white.withValues(alpha: 0.08),
                borderRadius: BorderRadius.circular(8),
              ),
            ),
            // Fill
            FractionallySizedBox(
              widthFactor: fraction,
              child: Container(
                height: 36,
                decoration: BoxDecoration(
                  color: isVoted
                      ? const Color(0xFFFF2D55).withValues(alpha: 0.4)
                      : Colors.white.withValues(alpha: 0.15),
                  borderRadius: BorderRadius.circular(8),
                ),
              ),
            ),
            // Label row
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 10),
              child: Row(
                children: [
                  if (isVoted)
                    const Padding(
                      padding: EdgeInsets.only(right: 6),
                      child: Icon(Icons.check_circle,
                          color: Color(0xFFFF2D55), size: 14),
                    ),
                  Expanded(
                    child: Text(
                      opt.text,
                      style: TextStyle(
                        color: isVoted ? const Color(0xFFFF2D55) : Colors.white,
                        fontSize: 13,
                        fontWeight: isVoted
                            ? FontWeight.bold
                            : FontWeight.normal,
                      ),
                    ),
                  ),
                  if (widget.votedOptionId != null || _isClosed)
                    Text(
                      '$percent%',
                      style: TextStyle(
                        color: isVoted
                            ? const Color(0xFFFF2D55)
                            : Colors.white70,
                        fontSize: 12,
                        fontWeight: FontWeight.bold,
                      ),
                    ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildFooter() {
    return Text(
      '${widget.poll.totalVotes} vote${widget.poll.totalVotes == 1 ? '' : 's'}',
      style: const TextStyle(color: Colors.white54, fontSize: 11),
    );
  }
}

class _CountdownChip extends StatelessWidget {
  const _CountdownChip({required this.secondsLeft});
  final int secondsLeft;

  @override
  Widget build(BuildContext context) {
    final isUrgent = secondsLeft <= 10;
    return AnimatedContainer(
      duration: const Duration(milliseconds: 300),
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
      decoration: BoxDecoration(
        color: isUrgent
            ? Colors.red.withValues(alpha: 0.8)
            : const Color(0xFFFF2D55).withValues(alpha: 0.7),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Text(
        '${secondsLeft}s',
        style: const TextStyle(
          color: Colors.white,
          fontWeight: FontWeight.bold,
          fontSize: 11,
        ),
      ),
    );
  }
}
