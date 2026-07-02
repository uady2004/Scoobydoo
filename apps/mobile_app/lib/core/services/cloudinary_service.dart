import 'package:dio/dio.dart';
import 'package:image_picker/image_picker.dart';

// ── Cloudinary Setup (free tier — takes ~2 min) ───────────────────────────────
// 1. Sign up at https://cloudinary.com  (25 GB storage, 25 GB/month CDN — free)
// 2. Copy your Cloud Name from the dashboard home page
// 3. Settings → Upload → Upload presets → Add upload preset
//      Signing mode: Unsigned  |  Folder: tiktok (or leave blank)
// 4. Replace cloudName and uploadPreset below with your values
// ─────────────────────────────────────────────────────────────────────────────

class CloudinaryService {
  static const String cloudName = 'jg4ipnzi';
  static const String uploadPreset = 'gmuvg31j';

  static bool get isConfigured =>
      cloudName != 'YOUR_CLOUD_NAME' && uploadPreset != 'YOUR_UPLOAD_PRESET';

  // Returns (url, thumbnailUrl) for the uploaded file.
  static Future<({String url, String thumbnailUrl})> uploadFile(
    XFile file, {
    void Function(int sent, int total)? onSendProgress,
  }) async {
    if (!isConfigured) {
      throw Exception(
        'Cloudinary is not configured.\n'
        'Open lib/core/services/cloudinary_service.dart and set your '
        'cloudName and uploadPreset.',
      );
    }

    final path = file.path.toLowerCase();
    final isVideo = path.endsWith('.mp4') ||
        path.endsWith('.mov') ||
        path.endsWith('.avi') ||
        path.endsWith('.mkv') ||
        (file.mimeType?.startsWith('video') ?? false);

    final resourceType = isVideo ? 'video' : 'image';
    final uploadUrl =
        'https://api.cloudinary.com/v1_1/$cloudName/$resourceType/upload';

    final formData = FormData.fromMap({
      'file': await MultipartFile.fromFile(file.path),
      'upload_preset': uploadPreset,
    });

    final dio = Dio();
    final response = await dio.post<Map<String, dynamic>>(
      uploadUrl,
      data: formData,
      onSendProgress: onSendProgress,
      options: Options(
        receiveTimeout: const Duration(minutes: 5),
        sendTimeout: const Duration(minutes: 5),
      ),
    );

    final data = response.data!;
    final secureUrl = data['secure_url'] as String;
    final publicId = data['public_id'] as String;

    final String thumbnailUrl;
    if (isVideo) {
      // Cloudinary auto-generates a JPEG thumbnail at the first frame.
      thumbnailUrl =
          'https://res.cloudinary.com/$cloudName/video/upload/so_0,w_400,h_711,c_fill/$publicId.jpg';
    } else {
      thumbnailUrl = secureUrl;
    }

    return (url: secureUrl, thumbnailUrl: thumbnailUrl);
  }
}
