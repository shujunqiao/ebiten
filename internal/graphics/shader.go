// Copyright 2022 The Ebiten Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package graphics

import (
	"bytes"
	"fmt"

	"github.com/hajimehoshi/ebiten/v2/internal/shader"
	"github.com/hajimehoshi/ebiten/v2/internal/shaderir"
)

func shaderSuffix(unit shaderir.Unit) (string, error) {
	shaderSuffix := fmt.Sprintf(`
var __imageDstTextureSize vec2

// imageSrcTextureSize returns the destination image's texture size in pixels.
func imageDstTextureSize() vec2 {
	return __imageDstTextureSize
}

var __imageSrcTextureSizes [%[1]d]vec2

// imageSrcTextureSize returns the 0th source image's texture size in pixels.
// As an image is a part of internal texture, the texture is usually bigger than the image.
// The texture's size is useful when you want to calculate pixels from texels in the texel mode.
func imageSrcTextureSize() vec2 {
	return __imageSrcTextureSizes[0]
}

// The unit is the source texture's pixel or texel.
var __imageDstRegionOrigin vec2

// The unit is the source texture's pixel or texel.
var __imageDstRegionSize vec2

// imageDstRegionOnTexture returns the destination image's region (the origin and the size) on its texture.
// The unit is the source texture's pixel or texel.
//
// As an image is a part of internal texture, the image can be located at an arbitrary position on the texture.
//
// Deprecated: as of v2.6. Use imageDstOrigin or imageDstSize.
func imageDstRegionOnTexture() (vec2, vec2) {
	return __imageDstRegionOrigin, __imageDstRegionSize
}

// imageDstRegionOnTexture returns the destination image's origin on its texture.
// The unit is the source texture's pixel or texel.
//
// As an image is a part of internal texture, the image can be located at an arbitrary position on the texture.
func imageDstOrigin() vec2 {
	return __imageDstRegionOrigin
}

// imageDstRegionOnTexture returns the destination image's size.
// The unit is the source texture's pixel or texel.
func imageDstSize() vec2 {
	return __imageDstRegionSize
}

// The unit is the source texture's pixel or texel.
var __imageSrcRegionOrigins [%[1]d]vec2

// The unit is the source texture's pixel or texel.
var __imageSrcRegionSizes [%[1]d]vec2

// imageSrcRegionOnTexture returns the 0th source image's region (the origin and the size) on its texture.
// The unit is the source texture's pixel or texel.
//
// As an image is a part of internal texture, the image can be located at an arbitrary position on the texture.
//
// Deprecated: as of v2.6. Use imageSrc0Origin or imageSrc0Size instead.
func imageSrcRegionOnTexture() (vec2, vec2) {
	return __imageSrcRegionOrigins[0], __imageSrcRegionSizes[0]
}
`, ShaderImageCount)

	for i := 0; i < ShaderImageCount; i++ {
		shaderSuffix += fmt.Sprintf(`
// imageSrc%[1]dOrigin returns the source image's region origin on its texture.
// The unit is the source texture's pixel or texel.
//
// As an image is a part of internal texture, the image can be located at an arbitrary position on the texture.
func imageSrc%[1]dOrigin() vec2 {
	return __imageSrcRegionOrigins[%[1]d]
}

// imageSrc%[1]dSize returns the source image's size.
// The unit is the source texture's pixel or texel.
func imageSrc%[1]dSize() vec2 {
	return __imageSrcRegionSizes[%[1]d]
}
`, i)

		pos := "pos"
		if i >= 1 {
			// Convert the position in texture0's positions to the target texture positions.
			switch unit {
			case shaderir.Pixels:
				pos = fmt.Sprintf("pos - __imageSrcRegionOrigins[0] + __imageSrcRegionOrigins[%d]", i)
			case shaderir.Texels:
				pos = fmt.Sprintf("((pos - __imageSrcRegionOrigins[0]) * __imageSrcTextureSizes[0]) / __imageSrcTextureSizes[%[1]d] + __imageSrcRegionOrigins[%[1]d]", i)
			default:
				return "", fmt.Errorf("graphics: unexpected unit: %d", unit)
			}
		}
		// __t%d is a special variable for a texture variable.
		shaderSuffix += fmt.Sprintf(`
func imageSrc%[1]dUnsafeAt(pos vec2) vec4 {
	// pos is the position in positions of the source texture (= 0th image's texture).
	return __texelAt(__t%[1]d, %[2]s)
}
`, i, pos)
		switch unit {
		case shaderir.Pixels:
			shaderSuffix += fmt.Sprintf(`
func imageSrc%[1]dAt(pos vec2) vec4 {
	// pos is the position of the source texture (= 0th image's texture).
	// If pos is in the region, the result is (1, 1). Otherwise, either element is 0.
	in := step(__imageSrcRegionOrigins[0], pos) - step(__imageSrcRegionOrigins[0] + __imageSrcRegionSizes[%[1]d], pos)
	return __texelAt(__t%[1]d, %[2]s) * in.x * in.y
}
`, i, pos)
		case shaderir.Texels:
			shaderSuffix += fmt.Sprintf(`
func imageSrc%[1]dAt(pos vec2) vec4 {
	// pos is the position of the source texture (= 0th image's texture).
	// If pos is in the region, the result is (1, 1). Otherwise, either element is 0.
	// With the texel mode, all the source region sizes are the same (#1870).
	// As pos is in texels of the 0th texture, always use the 0th image region size.
	in := step(__imageSrcRegionOrigins[0], pos) - step(__imageSrcRegionOrigins[0] + __imageSrcRegionSizes[0], pos)
	return __texelAt(__t%[1]d, %[2]s) * in.x * in.y
}
`, i, pos)
		}
	}

	shaderSuffix += `
var __projectionMatrix mat4

func __vertex(position vec2, texCoord vec2, color vec4) (vec4, vec2, vec4) {
	return __projectionMatrix * vec4(position, 0, 1), texCoord, color
}
`
	return shaderSuffix, nil
}

func CompileShader(src []byte) (*shaderir.Program, error) {
	unit, err := shader.ParseCompilerDirectives(src)
	if err != nil {
		return nil, err
	}
	suffix, err := shaderSuffix(unit)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.Write(src)
	buf.WriteString(suffix)

	const (
		vert = "__vertex"
		frag = "Fragment"
	)
	ir, err := shader.Compile(buf.Bytes(), vert, frag, ShaderImageCount)
	if err != nil {
		return nil, err
	}

	if ir.VertexFunc.Block == nil {
		return nil, fmt.Errorf("graphics: vertex shader entry point '%s' is missing", vert)
	}
	if ir.FragmentFunc.Block == nil {
		return nil, fmt.Errorf("graphics: fragment shader entry point '%s' is missing", frag)
	}

	return ir, nil
}
