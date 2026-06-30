import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:tiktok_clone/features/reports/data/datasources/report_remote_datasource.dart';

// ─────────────────────────────────────────────────────────────────────────────
// Constants
// ─────────────────────────────────────────────────────────────────────────────

const _kRed = Color(0xFFEE1D52);
const _kTeal = Color(0xFF25F4EE);
const _kSurface = Color(0xFF1C1C1E);

const List<({String value, String label})> _kReasons = [
  (value: 'spam', label: 'Spam or misleading'),
  (value: 'nudity', label: 'Nudity or sexual content'),
  (value: 'hate_speech', label: 'Hate speech or symbols'),
  (value: 'violence', label: 'Violence or dangerous content'),
  (value: 'bullying', label: 'Bullying or harassment'),
  (value: 'ip', label: 'Intellectual property violation'),
  (value: 'illegal_goods', label: 'Sale of illegal goods'),
  (value: 'self_harm', label: 'Self-harm or suicide'),
  (value: 'other', label: 'Other'),
];

// ─────────────────────────────────────────────────────────────────────────────
// Widget
// ─────────────────────────────────────────────────────────────────────────────

/// Full 3-step report flow: reason selection → optional details → confirmation.
class ReportScreen extends ConsumerStatefulWidget {
  const ReportScreen({
    super.key,
    required this.contentId,
    required this.contentType,
  });

  final String contentId;
  final String contentType;

  @override
  ConsumerState<ReportScreen> createState() => _ReportScreenState();
}

class _ReportScreenState extends ConsumerState<ReportScreen> {
  int _step = 1;
  String? _selectedReason;
  final TextEditingController _detailsCtrl = TextEditingController();
  bool _isSubmitting = false;

  @override
  void dispose() {
    _detailsCtrl.dispose();
    super.dispose();
  }

  // ── Submission ─────────────────────────────────────────────────────────────

  Future<void> _submit() async {
    if (_selectedReason == null) return;

    setState(() => _isSubmitting = true);

    try {
      await ref.read(reportDatasourceProvider).submitReport(
            contentType: widget.contentType,
            contentId: widget.contentId,
            reason: _selectedReason!,
            details: _detailsCtrl.text.trim().isEmpty
                ? null
                : _detailsCtrl.text.trim(),
          );
      if (mounted) setState(() => _step = 3);
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text(
              'Could not submit report. Please try again.',
              style: TextStyle(color: Colors.white),
            ),
            backgroundColor: _kSurface,
            behavior: SnackBarBehavior.floating,
          ),
        );
      }
    } finally {
      if (mounted) setState(() => _isSubmitting = false);
    }
  }

  // ── Step helpers ───────────────────────────────────────────────────────────

  void _goToStep2() {
    if (_selectedReason != null) setState(() => _step = 2);
  }

  void _backToStep1() => setState(() => _step = 1);

  // ── Gradient button decoration ─────────────────────────────────────────────

  BoxDecoration _redGradient(bool enabled) => BoxDecoration(
        borderRadius: BorderRadius.circular(4),
        gradient: enabled
            ? const LinearGradient(
                colors: [Color(0xFFEE1D52), Color(0xFFFF3B5C)],
                begin: Alignment.centerLeft,
                end: Alignment.centerRight,
              )
            : null,
        color: enabled ? null : Colors.grey[800],
      );

  // ── Builds ─────────────────────────────────────────────────────────────────

  @override
  Widget build(BuildContext context) {
    return switch (_step) {
      1 => _buildStep1(),
      2 => _buildStep2(),
      _ => _buildStep3(),
    };
  }

  // Step 1 — reason selection
  Widget _buildStep1() {
    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        backgroundColor: Colors.black,
        foregroundColor: Colors.white,
        elevation: 0,
        leading: IconButton(
          icon: const Icon(Icons.arrow_back_ios_new, color: Colors.white),
          onPressed: () => Navigator.of(context).pop(),
        ),
        title: const Text(
          'Report',
          style: TextStyle(
            color: Colors.white,
            fontWeight: FontWeight.w600,
            fontSize: 17,
          ),
        ),
        centerTitle: true,
      ),
      body: Column(
        children: [
          Expanded(
            child: RadioGroup<String>(
              groupValue: _selectedReason,
              onChanged: (v) => setState(() => _selectedReason = v),
              child: ListView.separated(
                padding: const EdgeInsets.symmetric(vertical: 8),
                itemCount: _kReasons.length,
                separatorBuilder: (_, __) => Divider(
                  color: Colors.white.withValues(alpha: 0.08),
                  height: 1,
                  indent: 16,
                  endIndent: 16,
                ),
                itemBuilder: (context, index) {
                  final reason = _kReasons[index];
                  final selected = _selectedReason == reason.value;
                  return InkWell(
                    onTap: () =>
                        setState(() => _selectedReason = reason.value),
                    splashColor: _kRed.withValues(alpha: 0.12),
                    highlightColor: Colors.transparent,
                    child: Container(
                      color: _kSurface,
                      padding: const EdgeInsets.symmetric(
                        horizontal: 16,
                        vertical: 12,
                      ),
                      child: Row(
                        children: [
                          Expanded(
                            child: Text(
                              reason.label,
                              style: TextStyle(
                                color:
                                    selected ? Colors.white : Colors.white70,
                                fontSize: 15,
                                fontWeight: selected
                                    ? FontWeight.w500
                                    : FontWeight.normal,
                              ),
                            ),
                          ),
                          Radio<String>(
                            value: reason.value,
                            activeColor: Colors.white,
                            fillColor:
                                WidgetStateProperty.resolveWith((states) {
                              if (states.contains(WidgetState.selected)) {
                                return Colors.white;
                              }
                              return Colors.white54;
                            }),
                            materialTapTargetSize:
                                MaterialTapTargetSize.shrinkWrap,
                            visualDensity: VisualDensity.compact,
                          ),
                        ],
                      ),
                    ),
                  );
                },
              ),
            ),
          ),
          _buildNextButton(),
        ],
      ),
    );
  }

  Widget _buildNextButton() {
    final enabled = _selectedReason != null;
    return SafeArea(
      top: false,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 20),
        child: GestureDetector(
          onTap: enabled ? _goToStep2 : null,
          child: Container(
            height: 50,
            decoration: _redGradient(enabled),
            alignment: Alignment.center,
            child: const Text(
              'Next',
              style: TextStyle(
                color: Colors.white,
                fontWeight: FontWeight.w600,
                fontSize: 15,
              ),
            ),
          ),
        ),
      ),
    );
  }

  // Step 2 — optional details + submit
  Widget _buildStep2() {
    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        backgroundColor: Colors.black,
        foregroundColor: Colors.white,
        elevation: 0,
        leading: IconButton(
          icon: const Icon(Icons.arrow_back_ios_new, color: Colors.white),
          onPressed: _backToStep1,
        ),
        title: const Text(
          'Add Details',
          style: TextStyle(
            color: Colors.white,
            fontWeight: FontWeight.w600,
            fontSize: 17,
          ),
        ),
        centerTitle: true,
      ),
      body: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Expanded(
            child: Padding(
              padding: const EdgeInsets.fromLTRB(16, 24, 16, 0),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  const Text(
                    'Tell us more (optional)',
                    style: TextStyle(
                      color: Colors.white,
                      fontSize: 16,
                      fontWeight: FontWeight.w500,
                    ),
                  ),
                  const SizedBox(height: 12),
                  TextField(
                    controller: _detailsCtrl,
                    maxLines: 5,
                    minLines: 5,
                    maxLength: 500,
                    style: const TextStyle(color: Colors.white, fontSize: 14),
                    cursorColor: _kRed,
                    decoration: InputDecoration(
                      hintText: 'Describe the issue...',
                      hintStyle: TextStyle(
                        color: Colors.white.withValues(alpha: 0.35),
                        fontSize: 14,
                      ),
                      filled: true,
                      fillColor: _kSurface,
                      counterStyle: TextStyle(
                        color: Colors.white.withValues(alpha: 0.4),
                        fontSize: 12,
                      ),
                      border: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(8),
                        borderSide: BorderSide.none,
                      ),
                      focusedBorder: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(8),
                        borderSide:
                            BorderSide(color: _kRed.withValues(alpha: 0.6), width: 1),
                      ),
                      contentPadding: const EdgeInsets.all(14),
                    ),
                  ),
                ],
              ),
            ),
          ),
          SafeArea(
            top: false,
            child: Padding(
              padding: const EdgeInsets.fromLTRB(16, 12, 16, 20),
              child: _isSubmitting
                  ? const Center(
                      child: CircularProgressIndicator(color: _kRed),
                    )
                  : GestureDetector(
                      onTap: _submit,
                      child: Container(
                        height: 50,
                        decoration: _redGradient(true),
                        alignment: Alignment.center,
                        child: const Text(
                          'Submit',
                          style: TextStyle(
                            color: Colors.white,
                            fontWeight: FontWeight.w600,
                            fontSize: 15,
                          ),
                        ),
                      ),
                    ),
            ),
          ),
        ],
      ),
    );
  }

  // Step 3 — success confirmation
  Widget _buildStep3() {
    return Scaffold(
      backgroundColor: Colors.black,
      body: SafeArea(
        child: Center(
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 32),
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                const Icon(
                  Icons.check_circle_rounded,
                  color: _kTeal,
                  size: 80,
                ),
                const SizedBox(height: 24),
                const Text(
                  'Report Submitted',
                  style: TextStyle(
                    color: Colors.white,
                    fontSize: 22,
                    fontWeight: FontWeight.bold,
                  ),
                ),
                const SizedBox(height: 12),
                const Text(
                  "Thank you for keeping TikTok safe.\nWe'll review your report within 24 hours.",
                  textAlign: TextAlign.center,
                  style: TextStyle(
                    color: Colors.grey,
                    fontSize: 14,
                    height: 1.5,
                  ),
                ),
                const SizedBox(height: 32),
                GestureDetector(
                  onTap: () => Navigator.of(context).pop(),
                  child: Container(
                    height: 50,
                    width: double.infinity,
                    decoration: _redGradient(true),
                    alignment: Alignment.center,
                    child: const Text(
                      'Done',
                      style: TextStyle(
                        color: Colors.white,
                        fontWeight: FontWeight.w600,
                        fontSize: 15,
                      ),
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
