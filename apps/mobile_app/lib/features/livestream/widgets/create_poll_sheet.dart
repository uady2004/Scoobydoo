import 'package:flutter/material.dart';

/// Bottom sheet for the host to create an in-stream poll.
class CreatePollSheet extends StatefulWidget {
  const CreatePollSheet({
    super.key,
    required this.onSubmit,
  });

  final Future<void> Function({
    required String question,
    required List<String> options,
    required int durationSecs,
  }) onSubmit;

  @override
  State<CreatePollSheet> createState() => _CreatePollSheetState();
}

class _CreatePollSheetState extends State<CreatePollSheet> {
  final _questionCtrl = TextEditingController();
  final List<TextEditingController> _optionCtrls = [
    TextEditingController(),
    TextEditingController(),
  ];
  int _durationSecs = 60;
  bool _submitting = false;

  static const _durations = [30, 60, 120, 300];

  @override
  void dispose() {
    _questionCtrl.dispose();
    for (final c in _optionCtrls) {
      c.dispose();
    }
    super.dispose();
  }

  bool get _isValid {
    if (_questionCtrl.text.trim().isEmpty) return false;
    final filled = _optionCtrls
        .where((c) => c.text.trim().isNotEmpty)
        .length;
    return filled >= 2;
  }

  void _addOption() {
    if (_optionCtrls.length >= 5) return;
    setState(() => _optionCtrls.add(TextEditingController()));
  }

  void _removeOption(int index) {
    if (_optionCtrls.length <= 2) return;
    setState(() {
      _optionCtrls[index].dispose();
      _optionCtrls.removeAt(index);
    });
  }

  Future<void> _submit() async {
    if (!_isValid || _submitting) return;
    setState(() => _submitting = true);
    try {
      final options = _optionCtrls
          .map((c) => c.text.trim())
          .where((t) => t.isNotEmpty)
          .toList();
      await widget.onSubmit(
        question: _questionCtrl.text.trim(),
        options: options,
        durationSecs: _durationSecs,
      );
      if (mounted) Navigator.pop(context);
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: EdgeInsets.only(
        left: 20,
        right: 20,
        top: 16,
        bottom: MediaQuery.of(context).viewInsets.bottom + 20,
      ),
      decoration: const BoxDecoration(
        color: Color(0xFF1A1A1A),
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      child: SingleChildScrollView(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            _buildHandle(),
            const Text(
              'Create Poll',
              style: TextStyle(
                color: Colors.white,
                fontSize: 18,
                fontWeight: FontWeight.bold,
              ),
            ),
            const SizedBox(height: 16),
            _buildQuestionField(),
            const SizedBox(height: 14),
            const Text('Options',
                style: TextStyle(color: Colors.white70, fontSize: 13)),
            const SizedBox(height: 8),
            ..._buildOptionFields(),
            if (_optionCtrls.length < 5)
              TextButton.icon(
                onPressed: _addOption,
                icon: const Icon(Icons.add, color: Color(0xFFFF2D55)),
                label: const Text('Add option',
                    style: TextStyle(color: Color(0xFFFF2D55))),
              ),
            const SizedBox(height: 12),
            _buildDurationSelector(),
            const SizedBox(height: 20),
            SizedBox(
              width: double.infinity,
              height: 48,
              child: ElevatedButton(
                onPressed: (_isValid && !_submitting) ? _submit : null,
                style: ElevatedButton.styleFrom(
                  backgroundColor: const Color(0xFFFF2D55),
                  disabledBackgroundColor: Colors.grey.shade800,
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(24),
                  ),
                ),
                child: _submitting
                    ? const SizedBox(
                        width: 22,
                        height: 22,
                        child: CircularProgressIndicator(
                          strokeWidth: 2,
                          color: Colors.white,
                        ),
                      )
                    : const Text(
                        'Launch Poll',
                        style: TextStyle(
                          color: Colors.white,
                          fontWeight: FontWeight.bold,
                          fontSize: 15,
                        ),
                      ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildHandle() {
    return Center(
      child: Container(
        width: 36,
        height: 4,
        margin: const EdgeInsets.only(bottom: 14),
        decoration: BoxDecoration(
          color: Colors.white.withValues(alpha: 0.3),
          borderRadius: BorderRadius.circular(2),
        ),
      ),
    );
  }

  Widget _buildQuestionField() {
    return TextField(
      controller: _questionCtrl,
      maxLength: 120,
      style: const TextStyle(color: Colors.white),
      decoration: InputDecoration(
        hintText: 'Ask your audience...',
        hintStyle: const TextStyle(color: Colors.white38),
        counterStyle: const TextStyle(color: Colors.white38),
        filled: true,
        fillColor: Colors.white.withValues(alpha: 0.08),
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(10),
          borderSide: BorderSide.none,
        ),
      ),
      onChanged: (_) => setState(() {}),
    );
  }

  List<Widget> _buildOptionFields() {
    return List.generate(_optionCtrls.length, (i) {
      return Padding(
        padding: const EdgeInsets.only(bottom: 8),
        child: Row(
          children: [
            Expanded(
              child: TextField(
                controller: _optionCtrls[i],
                maxLength: 60,
                style: const TextStyle(color: Colors.white),
                decoration: InputDecoration(
                  hintText: 'Option ${i + 1}',
                  hintStyle: const TextStyle(color: Colors.white38),
                  counterText: '',
                  filled: true,
                  fillColor: Colors.white.withValues(alpha: 0.08),
                  border: OutlineInputBorder(
                    borderRadius: BorderRadius.circular(10),
                    borderSide: BorderSide.none,
                  ),
                ),
                onChanged: (_) => setState(() {}),
              ),
            ),
            if (_optionCtrls.length > 2) ...[
              const SizedBox(width: 6),
              GestureDetector(
                onTap: () => _removeOption(i),
                child: const Icon(Icons.remove_circle_outline,
                    color: Colors.white38, size: 22),
              ),
            ],
          ],
        ),
      );
    });
  }

  Widget _buildDurationSelector() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const Text('Duration',
            style: TextStyle(color: Colors.white70, fontSize: 13)),
        const SizedBox(height: 8),
        Wrap(
          spacing: 8,
          children: _durations.map((secs) {
            final label = secs < 60
                ? '${secs}s'
                : '${secs ~/ 60}m';
            final selected = _durationSecs == secs;
            return ChoiceChip(
              label: Text(label),
              selected: selected,
              selectedColor: const Color(0xFFFF2D55),
              backgroundColor: Colors.white.withValues(alpha: 0.1),
              labelStyle: TextStyle(
                color: selected ? Colors.white : Colors.white70,
                fontWeight: selected ? FontWeight.bold : FontWeight.normal,
              ),
              onSelected: (_) => setState(() => _durationSecs = secs),
            );
          }).toList(),
        ),
      ],
    );
  }
}
