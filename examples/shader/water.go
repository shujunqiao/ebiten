// Copyright 2020 The Ebiten Authors
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

//go:build ignore

//kage:unit pixels

package main

var Time float
var Cursor vec2

func Fragment(position vec4, texCoord vec2, color vec4) vec4 {
	pos := position.xy - imageDstOrigin()

	border := imageDstSize().y*0.6 + 4*cos(Time*3+pos.y/10)
	if pos.y < border {
		return imageSrc2UnsafeAt(texCoord)
	}

	xoffset := 4 * cos(Time*3+pos.y/10)
	yoffset := 20 * (1 + cos(Time*3+pos.y/40))
	srcOrigin := imageSrc0Origin()
	clr := imageSrc2At(vec2(
		texCoord.x+xoffset,
		-(texCoord.y+yoffset-srcOrigin.y)+border*2+srcOrigin.y,
	)).rgb

	overlay := vec3(0.5, 1, 1)
	return vec4(mix(clr, overlay, 0.25), 1)
}
